package metricstore_client_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	metricstore_client "github.com/cloudfoundry/metric-store-release/src/pkg/client"
	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
	"google.golang.org/grpc"

	test "github.com/cloudfoundry/metric-store-release/src/pkg/testing"
)

func TestClientPromQLRange(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	client := metricstore_client.NewClient(metricStore.addr())
	hourAgo := time.Now().Truncate(time.Hour)

	result, err := client.PromQLRange(
		context.Background(),
		`some-query`,
		metricstore_client.WithPromQLStart(hourAgo),
		metricstore_client.WithPromQLEnd(hourAgo.Add(time.Minute)),
		metricstore_client.WithPromQLStep("5m"),
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	series := result.GetMatrix().GetSeries()
	if len(series) != 1 {
		t.Fatalf("expected to receive 1 series, got %d", len(series))
	}

	if series[0].GetPoints()[0].Value != 99 || series[0].GetPoints()[0].Time != 1234 {
		t.Fatalf("point[0] is incorrect; got %v", series[0].GetPoints()[0])
	}

	if series[0].GetPoints()[1].Value != 100 || series[0].GetPoints()[1].Time != 5678 {
		t.Fatalf("point[1] is incorrect; got %v", series[0].GetPoints()[1])
	}
	if len(metricStore.reqs) != 1 {
		t.Fatalf("expected have 1 request, have %d", len(metricStore.reqs))
	}

	if metricStore.reqs[0].URL.Path != "/api/v1/query_range" {
		t.Fatalf("expected Path '/api/v1/query_range' but got '%s'", metricStore.reqs[0].URL.Path)
	}

	assertQueryParam(t, metricStore.reqs[0].URL, "query", "some-query")
	assertQueryParam(t, metricStore.reqs[0].URL, "step", "5m")
	assertQueryParam(t, metricStore.reqs[0].URL, "start", test.FormatTimeWithDecimalMillis(hourAgo))
	assertQueryParam(t, metricStore.reqs[0].URL, "end", test.FormatTimeWithDecimalMillis(hourAgo.Add(time.Minute)))

	if len(metricStore.reqs[0].URL.Query()) != 4 {
		t.Fatalf("expected only a single query parameter, but got %d", len(metricStore.reqs[0].URL.Query()))
	}
}

func TestClientPromQL(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	client := metricstore_client.NewClient(metricStore.addr())

	result, err := client.PromQL(
		context.Background(),
		`some-query`,
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	samples := result.GetVector().GetSamples()
	if len(samples) != 1 {
		t.Fatalf("expected to receive 1 sample, got %d", len(samples))
	}

	if samples[0].Point.Value != 99 || samples[0].Point.Time != 1234 {
		t.Fatalf("samples[0].Point is incorrect; got %v", samples[0].GetPoint())
	}

	if len(metricStore.reqs) != 1 {
		t.Fatalf("expected have 1 request, have %d", len(metricStore.reqs))
	}

	if metricStore.reqs[0].URL.Path != "/api/v1/query" {
		t.Fatalf("expected Path '/api/v1/query' but got '%s'", metricStore.reqs[0].URL.Path)
	}

	assertQueryParam(t, metricStore.reqs[0].URL, "query", "some-query")

	if len(metricStore.reqs[0].URL.Query()) != 1 {
		t.Fatalf("expected only a single query parameter, but got %d", len(metricStore.reqs[0].URL.Query()))
	}
}

func TestClientPromQLWithOptions(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	client := metricstore_client.NewClient(metricStore.addr())

	_, err := client.PromQL(
		context.Background(),
		"some-query",
		metricstore_client.WithPromQLTime(time.Unix(101, 123000000)),
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	if len(metricStore.reqs) != 1 {
		t.Fatalf("expected have 1 request, have %d", len(metricStore.reqs))
	}

	if metricStore.reqs[0].URL.Path != "/api/v1/query" {
		t.Fatalf("expected Path '/api/v1/query' but got '%s'", metricStore.reqs[0].URL.Path)
	}

	assertQueryParam(t, metricStore.reqs[0].URL, "time", "101.123")

	if len(metricStore.reqs[0].URL.Query()) != 2 {
		t.Fatalf("expected 2 query parameters, but got %d", len(metricStore.reqs[0].URL.Query()))
	}
}

func TestClientPromQLNon200(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	metricStore.statusCode = 500
	client := metricstore_client.NewClient(metricStore.addr())

	_, err := client.PromQL(context.Background(), "some-query")

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientPromQLInvalidResponse(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	metricStore.result["GET/api/v1/query"] = []byte("invalid")
	client := metricstore_client.NewClient(metricStore.addr())

	_, err := client.PromQL(context.Background(), "some-query")

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientPromQLUnknownAddr(t *testing.T) {
	t.Parallel()
	client := metricstore_client.NewClient("http://invalid.url")

	_, err := client.PromQL(context.Background(), "some-query")

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientPromQLInvalidAddr(t *testing.T) {
	t.Parallel()
	client := metricstore_client.NewClient("-:-invalid")

	_, err := client.PromQL(context.Background(), "some-query")

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientPromQLCancelling(t *testing.T) {
	t.Parallel()
	metricStore := newStubMetricStore()
	metricStore.block = true
	client := metricstore_client.NewClient(metricStore.addr())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.PromQL(
		ctx,
		"some-query",
	)

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGrpcClientPromQL(t *testing.T) {
	t.Parallel()
	metricStore := newStubGrpcMetricStore()
	client := metricstore_client.NewClient(metricStore.addr(), metricstore_client.WithViaGRPC(grpc.WithInsecure()))

	result, err := client.PromQL(context.Background(), "some-query",
		metricstore_client.WithPromQLTime(time.Unix(99, 123000000)),
	)

	if err != nil {
		t.Fatal(err.Error())
	}

	scalar := result.GetScalar()
	if scalar.Time != 99 || scalar.Value != 101 {
		t.Fatalf("wrong scalar")
	}

	if len(metricStore.promInstantReqs) != 1 {
		t.Fatalf("expected have 1 request, have %d", len(metricStore.promInstantReqs))
	}

	if metricStore.promInstantReqs[0].Query != "some-query" {
		t.Fatalf("expected Query (%s) to equal %s", metricStore.promInstantReqs[0].Query, "some-query")
	}

	if metricStore.promInstantReqs[0].Time != "99.123" {
		t.Fatalf("expected Time (%s) to equal %s", metricStore.promInstantReqs[0].Time, "99.123")
	}
}

func TestGrpcClientPromQLCancelling(t *testing.T) {
	t.Parallel()
	metricStore := newStubGrpcMetricStore()
	metricStore.block = true
	client := metricstore_client.NewClient(metricStore.addr(), metricstore_client.WithViaGRPC(grpc.WithInsecure()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.PromQL(
		ctx,
		"some-query",
	)

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientAlwaysClosesBody(t *testing.T) {
	t.Parallel()

	spyHTTPClient := newSpyHTTPClient()
	client := metricstore_client.NewClient("", metricstore_client.WithHTTPClient(spyHTTPClient))
	client.PromQL(context.Background(), "some-query")

	if !spyHTTPClient.body.closed {
		t.Fatal("expected body to be closed")
	}
}

type stubMetricStore struct {
	statusCode int
	server     *httptest.Server
	reqs       []*http.Request
	bodies     [][]byte
	result     map[string][]byte
	block      bool
}

func newStubMetricStore() *stubMetricStore {
	s := &stubMetricStore{
		statusCode: http.StatusOK,
		result: map[string][]byte{
			"GET/api/v1/query": []byte(`
				{
				  "status": "success",
				  "data": {
					"resultType": "vector",
					"result": [
					  {
						"metric": {
						  "deployment": "cf"
						},
						"value": [ 1.234, "99" ]
					  }
					]
				  }
				}
			`),
			"GET/api/v1/query_range": []byte(`
				{
				  "status": "success",
				  "data": {
					"resultType": "matrix",
					"result": [
					  {
						"metric": {
						  "deployment": "cf"
						},
						"values": [
						  [ 1.234, "99" ],
						  [ 5.678, "100" ]
						]
					  }
					]
				  }
				}
			`),
		},
	}
	s.server = httptest.NewServer(s)
	return s
}

func (s *stubMetricStore) addr() string {
	return s.server.URL
}

func (s *stubMetricStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.block {
		var block chan struct{}
		<-block
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	s.bodies = append(s.bodies, body)
	s.reqs = append(s.reqs, r)
	w.WriteHeader(s.statusCode)
	w.Write(s.result[r.Method+r.URL.Path])
}

func assertQueryParam(t *testing.T, u *url.URL, name string, values ...string) {
	t.Helper()
	for _, value := range values {
		var found bool
		for _, actual := range u.Query()[name] {
			if actual == value {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("expected query parameter '%s' to contain '%s', but got '%v'", name, value, u.Query()[name])
		}
	}
}

type stubGrpcMetricStore struct {
	mu              sync.Mutex
	promInstantReqs []*rpc.PromQL_InstantQueryRequest
	promRangeReqs   []*rpc.PromQL_RangeQueryRequest
	lis             net.Listener
	block           bool
}

func newStubGrpcMetricStore() *stubGrpcMetricStore {
	s := &stubGrpcMetricStore{}
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	s.lis = lis
	srv := grpc.NewServer()
	rpc.RegisterPromQLAPIServer(srv, s)
	go srv.Serve(lis)

	return s
}

func (s *stubGrpcMetricStore) addr() string {
	return s.lis.Addr().String()
}

func (s *stubGrpcMetricStore) InstantQuery(c context.Context, r *rpc.PromQL_InstantQueryRequest) (*rpc.PromQL_InstantQueryResult, error) {
	if s.block {
		var block chan struct{}
		<-block
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.promInstantReqs = append(s.promInstantReqs, r)

	return &rpc.PromQL_InstantQueryResult{
		Result: &rpc.PromQL_InstantQueryResult_Scalar{
			Scalar: &rpc.PromQL_Point{
				Time:  99,
				Value: 101,
			},
		},
	}, nil
}

func (s *stubGrpcMetricStore) RangeQuery(c context.Context, r *rpc.PromQL_RangeQueryRequest) (*rpc.PromQL_RangeQueryResult, error) {
	if s.block {
		var block chan struct{}
		<-block
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.promRangeReqs = append(s.promRangeReqs, r)

	return &rpc.PromQL_RangeQueryResult{
		Result: &rpc.PromQL_RangeQueryResult_Matrix{
			Matrix: &rpc.PromQL_Matrix{
				Series: []*rpc.PromQL_Series{
					{
						Metric: map[string]string{
							"__name__": "test",
						},
						Points: []*rpc.PromQL_Point{
							{
								Time:  99,
								Value: 101,
							},
						},
					},
				},
			},
		},
	}, nil
}

func (s *stubGrpcMetricStore) SeriesQuery(ctx context.Context, req *rpc.PromQL_SeriesQueryRequest) (*rpc.PromQL_SeriesQueryResult, error) {
	panic("stub :(")
}

func (s *stubGrpcMetricStore) LabelsQuery(ctx context.Context, req *rpc.PromQL_LabelsQueryRequest) (*rpc.PromQL_LabelsQueryResult, error) {
	panic("stub :(")
}

func (s *stubGrpcMetricStore) LabelValuesQuery(ctx context.Context, req *rpc.PromQL_LabelValuesQueryRequest) (*rpc.PromQL_LabelValuesQueryResult, error) {
	panic("stub :(")
}

type stubBufferCloser struct {
	*bytes.Buffer
	closed bool
}

func newStubBufferCloser() *stubBufferCloser {
	return &stubBufferCloser{}
}

func (s *stubBufferCloser) Close() error {
	s.closed = true
	return nil
}

type spyHTTPClient struct {
	body *stubBufferCloser
}

func newSpyHTTPClient() *spyHTTPClient {
	return &spyHTTPClient{
		body: newStubBufferCloser(),
	}
}

func (s *spyHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Body: s.body,
	}, nil
}
