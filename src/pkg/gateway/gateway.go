package gateway

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	rpc "github.com/cloudfoundry/metric-store-release/src/pkg/rpc/metricstore_v1"
)

// Gateway provides a RESTful API into MetricStore's gRPC API.
type Gateway struct {
	log *log.Logger

	metricStoreAddr string

	gatewayAddr         string
	lis                 net.Listener
	blockOnStart        bool
	metricStoreDialOpts []grpc.DialOption
	certPath            string
	keyPath             string
}

// NewGateway creates a new Gateway. It will listen on the gatewayAddr and
// submit requests via gRPC to the MetricStore on metricStoreAddr. Start() must be
// invoked before using the Gateway.
func NewGateway(metricStoreAddr, gatewayAddr, certPath, keyPath string, opts ...GatewayOption) *Gateway {
	g := &Gateway{
		log:             log.New(ioutil.Discard, "", 0),
		metricStoreAddr: metricStoreAddr,
		gatewayAddr:     gatewayAddr,
		certPath:        certPath,
		keyPath:         keyPath,
	}

	for _, o := range opts {
		o(g)
	}

	return g
}

// GatewayOption configures a Gateway.
type GatewayOption func(*Gateway)

// WithGatewayLogger returns a GatewayOption that configures the logger for
// the Gateway. It defaults to no logging.
func WithGatewayLogger(l *log.Logger) GatewayOption {
	return func(g *Gateway) {
		g.log = l
	}
}

// WithGatewayBlock returns a GatewayOption that determines if Start launches
// a go-routine or not. It defaults to launching a go-routine. If this is set,
// start will block on serving the HTTP endpoint.
func WithGatewayBlock() GatewayOption {
	return func(g *Gateway) {
		g.blockOnStart = true
	}
}

// WithGatewayMetricStoreDialOpts returns a GatewayOption that sets grpc.DialOptions on the
// metric-store dial
func WithGatewayMetricStoreDialOpts(opts ...grpc.DialOption) GatewayOption {
	return func(g *Gateway) {
		g.metricStoreDialOpts = opts
	}
}

// Start starts the gateway to start receiving and forwarding requests. It
// does not block unless WithGatewayBlock was set.
func (g *Gateway) Start() {
	lis, err := net.Listen("tcp", g.gatewayAddr)
	if err != nil {
		g.log.Fatalf("failed to listen on addr %s: %s", g.gatewayAddr, err)
	}
	g.lis = lis
	g.log.Printf("listening on %s...", lis.Addr().String())

	if g.blockOnStart {
		g.listenAndServe()
		return
	}

	go g.listenAndServe()
}

// Addr returns the address the gateway is listening on. Start must be called
// first.
func (g *Gateway) Addr() string {
	return g.lis.Addr().String()
}

func (g *Gateway) listenAndServe() {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true},
		),
	)

	runtime.HTTPError = g.httpErrorHandler

	conn, err := grpc.Dial(g.metricStoreAddr, g.metricStoreDialOpts...)
	if err != nil {
		g.log.Fatalf("failed to dial Metric Store: %s", err)
	}

	err = rpc.RegisterPromQLAPIHandlerClient(
		context.Background(),
		mux,
		rpc.NewPromQLAPIClient(conn),
	)
	if err != nil {
		g.log.Fatalf("failed to register PromQLAPI handler: %s", err)
	}

	server := &http.Server{Handler: mux}
	if err := server.ServeTLS(g.lis, g.certPath, g.keyPath); err != nil {
		g.log.Fatalf("failed to serve HTTPS endpoint: %s", err)
	}
}

type errorBody struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (g *Gateway) httpErrorHandler(
	ctx context.Context,
	mux *runtime.ServeMux,
	marshaler runtime.Marshaler,
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	const fallback = `{"error": "failed to marshal error message"}`

	w.Header().Del("Trailer")
	w.Header().Set("Content-Type", marshaler.ContentType())

	body := &errorBody{
		Status:    "error",
		ErrorType: "internal",
		Error:     grpc.ErrorDesc(err),
	}

	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		g.log.Printf("Failed to marshal error message %q: %v", body, merr)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			g.log.Printf("Failed to write response: %v", err)
		}
		return
	}

	w.WriteHeader(runtime.HTTPStatusFromCode(grpc.Code(err)))
	if _, err := w.Write(buf); err != nil {
		g.log.Printf("Failed to write response: %v", err)
	}
}
