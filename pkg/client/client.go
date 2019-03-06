package metricstore_client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	rpc "github.com/cloudfoundry/metric-store/pkg/rpc/metricstore_v1"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
)

// Client reads from MetricStore via the RESTful or gRPC API.
type Client struct {
	addr string

	httpClient HTTPClient
	grpcClient rpc.PromQLAPIClient
}

// NewIngressClient creates a Client.
func NewClient(addr string, opts ...ClientOption) *Client {
	c := &Client{
		addr: addr,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	for _, o := range opts {
		o.configure(c)
	}

	return c
}

// ClientOption configures the MetricStore client.
type ClientOption interface {
	configure(client interface{})
}

// clientOptionFunc enables regular functions to be a ClientOption.
type clientOptionFunc func(client interface{})

// configure Implements clientOptionFunc.
func (f clientOptionFunc) configure(client interface{}) {
	f(client)
}

// HTTPClient is an interface that represents a http.Client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// WithHTTPClient sets the HTTP client. It defaults to a client that timesout
// after 5 seconds.
func WithHTTPClient(h HTTPClient) ClientOption {
	return clientOptionFunc(func(c interface{}) {
		switch c := c.(type) {
		case *Client:
			c.httpClient = h
		default:
			panic("unknown type")
		}
	})
}

// WithViaGRPC enables gRPC instead of HTTP/1 for reading from MetricStore.
func WithViaGRPC(opts ...grpc.DialOption) ClientOption {
	return clientOptionFunc(func(c interface{}) {
		switch c := c.(type) {
		case *Client:
			conn, err := grpc.Dial(c.addr, opts...)
			if err != nil {
				panic(fmt.Sprintf("failed to dial via gRPC: %s", err))
			}

			c.grpcClient = rpc.NewPromQLAPIClient(conn)
		default:
			panic("unknown type")
		}
	})
}

// PromQLOption configures the URL that is used to submit the query. The
// RawQuery is set to the decoded query parameters after each option is
// invoked.
type PromQLOption func(u *url.URL, q url.Values)

// WithPromQLTime returns a PromQLOption that configures the 'time' query
// parameter for a PromQL query.
func WithPromQLTime(t time.Time) PromQLOption {
	return func(u *url.URL, q url.Values) {
		q.Set("time", formatTimeWithDecimalMillis(t))
	}
}

func WithPromQLStart(t time.Time) PromQLOption {
	return func(u *url.URL, q url.Values) {
		q.Set("start", formatTimeWithDecimalMillis(t))
	}
}

func WithPromQLEnd(t time.Time) PromQLOption {
	return func(u *url.URL, q url.Values) {
		q.Set("end", formatTimeWithDecimalMillis(t))
	}
}

func formatTimeWithDecimalMillis(t time.Time) string {
	return fmt.Sprintf("%.3f", float64(t.UnixNano())/1e9)
}

func WithPromQLStep(step string) PromQLOption {
	return func(u *url.URL, q url.Values) {
		q.Set("step", step)
	}
}

// PromQL issues a PromQL query against Metric Store data.
func (c *Client) PromQLRange(
	ctx context.Context,
	query string,
	opts ...PromQLOption,
) (*rpc.PromQL_RangeQueryResult, error) {
	if c.grpcClient != nil {
		return c.grpcPromQLRange(ctx, query, opts)
	}

	u, err := url.Parse(c.addr)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/v1/query_range"
	q := u.Query()
	q.Set("query", query)

	// allow the given options to configure the URL.
	for _, o := range opts {
		o(u, q)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var promQLResponse rpc.PromQL_RangeQueryResult
	marshaler := &runtime.JSONPb{}
	if err := marshaler.NewDecoder(resp.Body).Decode(&promQLResponse); err != nil {
		return nil, err
	}

	return &promQLResponse, nil
}

func (c *Client) grpcPromQLRange(ctx context.Context, query string, opts []PromQLOption) (*rpc.PromQL_RangeQueryResult, error) {
	u := &url.URL{}
	q := u.Query()
	// allow the given options to configure the URL.
	for _, o := range opts {
		o(u, q)
	}

	req := &rpc.PromQL_RangeQueryRequest{
		Query: query,
	}

	if v, ok := q["start"]; ok {
		req.Start = v[0]
	}

	if v, ok := q["end"]; ok {
		req.End = v[0]
	}

	if v, ok := q["step"]; ok {
		req.Step = v[0]
	}

	resp, err := c.grpcClient.RangeQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// PromQL issues a PromQL query against Metric Store data.
func (c *Client) PromQL(
	ctx context.Context,
	query string,
	opts ...PromQLOption,
) (*rpc.PromQL_InstantQueryResult, error) {
	if c.grpcClient != nil {
		return c.grpcPromQL(ctx, query, opts)
	}

	u, err := url.Parse(c.addr)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/v1/query"
	q := u.Query()
	q.Set("query", query)

	// allow the given options to configure the URL.
	for _, o := range opts {
		o(u, q)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var promQLResponse rpc.PromQL_InstantQueryResult
	marshaler := &runtime.JSONPb{}
	if err := marshaler.NewDecoder(resp.Body).Decode(&promQLResponse); err != nil {
		return nil, err
	}

	return &promQLResponse, nil
}

func (c *Client) grpcPromQL(ctx context.Context, query string, opts []PromQLOption) (*rpc.PromQL_InstantQueryResult, error) {
	u := &url.URL{}
	q := u.Query()
	// allow the given options to configure the URL.
	for _, o := range opts {
		o(u, q)
	}

	req := &rpc.PromQL_InstantQueryRequest{
		Query: query,
	}

	if v, ok := q["time"]; ok {
		req.Time = v[0]
	}

	resp, err := c.grpcClient.InstantQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
