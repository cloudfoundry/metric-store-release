package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
)

const (
	MAX_RETRIES = 3
)

type CAPIClient struct {
	client                  HTTPClient
	externalCapi            string
	tokenCache              *sync.Map
	tokenPruningInterval    time.Duration
	cacheExpirationInterval time.Duration
	log                     *logger.Logger
	metrics                 metrics.Registrar
}

func NewCAPIClient(
	externalCapiAddr string,
	client HTTPClient,
	metrics metrics.Registrar,
	log *logger.Logger,
	opts ...CAPIOption,
) *CAPIClient {
	_, err := url.Parse(externalCapiAddr)
	if err != nil {
		log.Fatal("failed to parse external CAPI addr", err)
	}

	c := &CAPIClient{
		client:                  client,
		externalCapi:            externalCapiAddr,
		tokenCache:              &sync.Map{},
		tokenPruningInterval:    time.Minute,
		cacheExpirationInterval: time.Minute,
		log:                     log,
		metrics:                 metrics,
	}

	for _, opt := range opts {
		opt(c)
	}

	go c.pruneTokens()

	return c
}

type CAPIOption func(c *CAPIClient)

func WithTokenPruningInterval(interval time.Duration) CAPIOption {
	return func(c *CAPIClient) {
		c.tokenPruningInterval = interval
	}
}

func WithCacheExpirationInterval(interval time.Duration) CAPIOption {
	return func(c *CAPIClient) {
		c.cacheExpirationInterval = interval
	}
}

type authorizedSourceIds struct {
	sourceIds  []string
	expiresAt  time.Time
}

func (c *CAPIClient) IsAuthorized(sourceId string, clientToken string) bool {
	var sourceIds []string
	s, ok := c.tokenCache.Load(clientToken)

	// if the token was found in the cache and hasn't expired yet, we'll
	// check to see if the sourceId is contained
	if ok && time.Now().Before(s.(authorizedSourceIds).expiresAt) {
		sourceIds = s.(authorizedSourceIds).sourceIds

		// if our cache contains the sourceId, then we're all set
		if isContained(sourceIds, sourceId) {
			return true
		}
	}

	// if we are here, one of two scenarios is possible:
	// 1) we didn't find the token in the cache, so we're fetching from
	//    CAPI for the very first time for this token
	// 2) we found the token in the cache, but failed to find the sourceId
	//    and are under our retry limit, thus we want to ask CAPI for a
	//    refreshed list of sourceIds

	//sourceIds = c.AvailableSourceIDs(clientToken)
	guId, err := c.CheckAvailableSourceID(sourceId, clientToken)
	if err != nil {
		c.log.Error("failed to build authorize log access request", err)
		return false
	}
	sourceIds = append(sourceIds, guId)

	c.tokenCache.Store(clientToken, authorizedSourceIds{
		sourceIds:  sourceIds,
		expiresAt:  time.Now().Add(c.cacheExpirationInterval),
	})

	return isContained(sourceIds, sourceId)
}

func isContained(sourceIds []string, sourceId string) bool {
	for _, s := range sourceIds {
		if s == sourceId {
			return true
		}
	}

	return false
}

func (c *CAPIClient) AvailableSourceIDs(authToken string) []string {
	var sourceIDs []string
	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps", nil)
	if err != nil {
		c.log.Error("failed to build authorize log access request", err)
		return nil
	}

	resources, err := c.doPaginatedResourceRequest(req, authToken)
	if err != nil {
		c.log.Error("failed to make request", err)
		return nil
	}
	for _, resource := range resources {
		sourceIDs = append(sourceIDs, resource.Guid)
	}

	req, err = http.NewRequest(http.MethodGet, c.externalCapi+"/v3/service_instances", nil)
	if err != nil {
		c.log.Error("failed to build access request", err)
		return nil
	}

	resources, err = c.doPaginatedResourceRequest(req, authToken)
	if err != nil {
		c.log.Error("failed to make request", err)
		return nil
	}
	for _, resource := range resources {
		sourceIDs = append(sourceIDs, resource.Guid)
	}

	return sourceIDs
}

func (c *CAPIClient) CheckAvailableSourceID(sourceId string, authToken string) (string, error) {
	var guId string
	if len(sourceId) == 0 {
		return guId, fmt.Errorf("failed: the sourceId is not provided")
	}

	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps/"+sourceId, nil)
	if err != nil {
		c.log.Error("failed to build authorize log access request", err)
		return guId, err
	}

	resource, err := c.doGuidAndNameRequest(req, authToken)
	if err == nil {
		return resource.Guid, err
	}
	c.log.Error("The sourceId was not found in /v3/apps request", err)

	req, err = http.NewRequest(http.MethodGet, c.externalCapi+"/v3/service_instances/"+sourceId, nil)
	if err != nil {
		c.log.Error("failed to build access request", err)
		return guId, err
	}

	resource, err = c.doGuidAndNameRequest(req, authToken)
	if err != nil {
		c.log.Error("failed to make request", err)
		return guId, err
	}
	return resource.Guid, nil
}

func (c *CAPIClient) GetRelatedSourceIds(appNames []string, authToken string) map[string][]string {
	if len(appNames) == 0 {
		return map[string][]string{}
	}

	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps", nil)
	if err != nil {
		c.log.Error("failed to build app list request", err)
		return map[string][]string{}
	}

	query := req.URL.Query()
	query.Set("names", strings.Join(appNames, ","))
	query.Set("per_page", "5000")
	req.URL.RawQuery = query.Encode()

	guidSets := make(map[string][]string)

	resources, err := c.doPaginatedResourceRequest(req, authToken)
	if err != nil {
		c.log.Error("failed to make request", err)
		return map[string][]string{}
	}
	for _, resource := range resources {
		guidSets[resource.Name] = append(guidSets[resource.Name], resource.Guid)
	}

	return guidSets
}

type resource struct {
	Guid string `json:"guid"`
	Name string `json:"name"`
}

func (c *CAPIClient) doPaginatedResourceRequest(req *http.Request, authToken string) ([]resource, error) {
	var resources []resource

	for {
		page, nextPageURL, err := c.doResourceRequest(req, authToken)
		if err != nil {
			return nil, err
		}

		resources = append(resources, page...)

		if nextPageURL == nil {
			break
		}
		req.URL = nextPageURL
	}

	return resources, nil
}

func (c *CAPIClient) doResourceRequest(req *http.Request, authToken string) ([]resource, *url.URL, error) {
	resp, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed CAPI request (%s) with error: %s", req.URL.Path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf(
			"failed CAPI request (%s) with status: %d (%s)",
			req.URL.Path,
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
		)
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	var apps struct {
		Pagination struct {
			Next struct {
				Href string `json:"href"`
			} `json:"next"`
		} `json:"pagination"`
		Resources []resource `json:"resources"`
	}

	err = json.NewDecoder(resp.Body).Decode(&apps)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode resource list request (%s): %s", req.URL.Path, err)
	}

	var nextPageURL *url.URL
	if apps.Pagination.Next.Href != "" {
		nextPageURL, err = url.Parse(apps.Pagination.Next.Href)
		if err != nil {
			return apps.Resources, nextPageURL, fmt.Errorf("failed to parse URL %s: %s", apps.Pagination.Next.Href, err)
		}
		nextPageURL.Scheme, nextPageURL.Host = req.URL.Scheme, req.URL.Host
	}

	return apps.Resources, nextPageURL, nil
}

func (c *CAPIClient) doGuidAndNameRequest(req *http.Request, authToken string) (resource, error) {
	var Resource resource
	resp, err := c.doRequest(req, authToken)
	if err != nil {
		return Resource, fmt.Errorf("failed CAPI request (%s) with error: %s", req.URL.Path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Resource, fmt.Errorf(
			"failed CAPI request (%s) with status: %d (%s)",
			req.URL.Path,
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
		)
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	err = json.NewDecoder(resp.Body).Decode(&Resource)
	if err != nil {
		return Resource, fmt.Errorf("failed to decode resource list request (%s): %s", req.URL.Path, err)
	}

	return Resource, nil
}

func (c *CAPIClient) TokenCacheSize() int {
	var i int
	c.tokenCache.Range(func(_, _ interface{}) bool {
		i++
		return true
	})
	return i
}

func (c *CAPIClient) pruneTokens() {
	for range time.Tick(c.tokenPruningInterval) {
		now := time.Now()

		c.tokenCache.Range(func(k, v interface{}) bool {
			cachedSources := v.(authorizedSourceIds)

			if now.After(cachedSources.expiresAt) {
				c.tokenCache.Delete(k)
			}

			return true
		})
	}
}

func (c *CAPIClient) doRequest(req *http.Request, authToken string) (*http.Response, error) {
	req.Header.Set("Authorization", authToken)
	start := time.Now()
	resp, err := c.client.Do(req)
	c.metrics.Histogram(metrics.AuthProxyCAPIRequestDurationSeconds).Observe(float64(time.Since(start).Seconds()))

	if err != nil {
		c.log.Error("CAPI request failed", err, logger.String("url", req.URL.Path))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Info("CAPI request returned non-200",
			logger.String("url", req.URL.Path),
			logger.String("status_code", strconv.Itoa(resp.StatusCode)),
		)
		cleanup(resp)
		return resp, nil
	}

	return resp, nil
}

func cleanup(resp *http.Response) {
	if resp == nil {
		return
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}
