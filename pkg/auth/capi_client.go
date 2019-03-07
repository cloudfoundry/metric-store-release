package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type CAPIClient struct {
	client                  HTTPClient
	externalCapi            string
	tokenCache              *sync.Map
	tokenPruningInterval    time.Duration
	cacheExpirationInterval time.Duration

	storeAppsLatency                 func(float64)
	storeListServiceInstancesLatency func(float64)
	storeAppsByNameLatency           func(float64)
}

func NewCAPIClient(
	externalCapiAddr string,
	client HTTPClient,
	m Metrics,
	log *log.Logger,
	opts ...CAPIOption,
) *CAPIClient {
	_, err := url.Parse(externalCapiAddr)
	if err != nil {
		log.Fatalf("failed to parse external CAPI addr: %s", err)
	}

	c := &CAPIClient{
		client:                  client,
		externalCapi:            externalCapiAddr,
		tokenCache:              &sync.Map{},
		tokenPruningInterval:    time.Minute,
		cacheExpirationInterval: time.Minute,

		storeAppsLatency:                 m.NewGauge("cf_auth_proxy_last_capiv3_apps_latency", "nanoseconds"),
		storeListServiceInstancesLatency: m.NewGauge("cf_auth_proxy_last_capiv3_list_service_instances_latency", "nanoseconds"),
		storeAppsByNameLatency:           m.NewGauge("cf_auth_proxy_last_capiv3_apps_by_name_latency", "nanoseconds"),
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
	sourceIds []string
	expiresAt time.Time
}

func (c *CAPIClient) IsAuthorized(sourceId string, clientToken string) bool {
	var sourceIds []string
	s, ok := c.tokenCache.Load(clientToken)

	if ok && time.Now().Before(s.(authorizedSourceIds).expiresAt) {
		sourceIds = s.(authorizedSourceIds).sourceIds
	} else {
		sourceIds = c.AvailableSourceIds(clientToken)

		c.tokenCache.Store(clientToken, authorizedSourceIds{
			sourceIds: sourceIds,
			expiresAt: time.Now().Add(c.cacheExpirationInterval),
		})
	}

	for _, s := range sourceIds {
		if s == sourceId {
			return true
		}
	}

	return false
}

func (c *CAPIClient) AvailableSourceIds(authToken string) []string {
	// TODO - this method can't even return an error, we're begging to cache
	// an empty list
	var sourceIds []string
	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps", nil)
	if err != nil {
		log.Printf("failed to build authorize log access request: %s", err)
		return nil
	}

	preserveScheme, preserveHost := req.URL.Scheme, req.URL.Host
	for {
		resources, nextPageURL, err := c.doResourceRequest(req, authToken, c.storeAppsLatency)
		if err != nil {
			log.Print(err)
			return nil
		}

		for _, resource := range resources {
			sourceIds = append(sourceIds, resource.Guid)
		}

		if nextPageURL == nil {
			break
		}

		req.URL = nextPageURL
		req.URL.Scheme, req.URL.Host = preserveScheme, preserveHost
	}

	req, err = http.NewRequest(http.MethodGet, c.externalCapi+"/v3/service_instances", nil)
	if err != nil {
		log.Printf("failed to build authorize service instance access request: %s", err)
		return nil
	}

	for {
		resources, nextPageURL, err := c.doResourceRequest(req, authToken, c.storeListServiceInstancesLatency)
		if err != nil || resources == nil {
			log.Print(err)
			return nil
		}

		for _, resource := range resources {
			sourceIds = append(sourceIds, resource.Guid)
		}

		if nextPageURL == nil {
			break
		}
		req.URL = nextPageURL
	}

	return sourceIds
}

func (c *CAPIClient) GetRelatedSourceIds(appNames []string, authToken string) map[string][]string {
	if len(appNames) == 0 {
		return map[string][]string{}
	}

	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps", nil)
	if err != nil {
		log.Printf("failed to build app list request: %s", err)
		return map[string][]string{}
	}

	query := req.URL.Query()
	query.Set("names", strings.Join(appNames, ","))
	query.Set("per_page", "5000")
	req.URL.RawQuery = query.Encode()

	guidSets := make(map[string][]string)

	for {
		resources, nextPageURL, err := c.doResourceRequest(req, authToken, c.storeAppsByNameLatency)
		if err != nil {
			log.Print(err)
			return map[string][]string{}
		}

		for _, resource := range resources {
			guidSets[resource.Name] = append(guidSets[resource.Name], resource.Guid)
		}

		if nextPageURL == nil {
			break
		}

		req.URL = nextPageURL
	}

	return guidSets
}

type resource struct {
	Guid string `json:"guid"`
	Name string `json:"name"`
}

func (c *CAPIClient) doResourceRequest(req *http.Request, authToken string, metric func(float64)) ([]resource, *url.URL, error) {
	resp, err := c.doRequest(req, authToken, metric)
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
	}

	return apps.Resources, nextPageURL, nil
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

func (c *CAPIClient) doRequest(req *http.Request, authToken string, reporter func(float64)) (*http.Response, error) {
	req.Header.Set("Authorization", authToken)
	start := time.Now()
	resp, err := c.client.Do(req)
	reporter(float64(time.Since(start)))

	if err != nil {
		log.Printf("CAPI request (%s) failed: %s", req.URL.Path, err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("CAPI request (%s) returned: %d", req.URL.Path, resp.StatusCode)
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
