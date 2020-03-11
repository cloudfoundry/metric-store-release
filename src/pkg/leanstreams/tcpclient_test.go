package leanstreams_test

import (
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	shared "github.com/cloudfoundry/metric-store-release/src/internal/testing"
	. "github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams/test/message"
	sharedtls "github.com/cloudfoundry/metric-store-release/src/internal/tls"
	"github.com/golang/protobuf/proto"
)

func exampleCallback(bts []byte) error {
	msg := &message.Note{}
	err := proto.Unmarshal(bts, msg)
	return err
}

var (
	tlsServerConfig, _ = sharedtls.NewMutualTLSServerConfig(
		shared.Cert("metric-store-ca.crt"),
		shared.Cert("metric-store.crt"),
		shared.Cert("metric-store.key"),
	)
	tlsClientConfig, _ = sharedtls.NewMutualTLSClientConfig(
		shared.Cert("metric-store-ca.crt"),
		shared.Cert("metric-store.crt"),
		shared.Cert("metric-store.key"),
		"metric-store",
	)

	buffWriteConfig = TCPClientConfig{
		MaxMessageSize: 2048,
		Address:        FormatAddress("127.0.0.1", strconv.Itoa(5034)),
		TLSConfig:      tlsClientConfig,
	}

	buffWriteConfig2 = TCPClientConfig{
		MaxMessageSize: 2048,
		Address:        FormatAddress("127.0.0.1", strconv.Itoa(5035)),
		TLSConfig:      tlsClientConfig,
	}

	listenConfig = TCPListenerConfig{
		MaxMessageSize: 2048,
		Logger:         log.New(os.Stdout, "leanstreams", log.LstdFlags),
		Address:        FormatAddress("", strconv.Itoa(5033)),
		Callback:       exampleCallback,
		TLSConfig:      tlsServerConfig,
	}

	listenConfig2 = TCPListenerConfig{
		MaxMessageSize: 2048,
		Logger:         log.New(os.Stdout, "leanstreams", log.LstdFlags),
		Address:        FormatAddress("", strconv.Itoa(5034)),
		Callback:       exampleCallback,
		TLSConfig:      tlsServerConfig,
	}

	listenConfig3 = TCPListenerConfig{
		MaxMessageSize: 2048,
		Logger:         log.New(os.Stdout, "leanstreams", log.LstdFlags),
		Address:        FormatAddress("", strconv.Itoa(5035)),
		Callback:       exampleCallback,
		TLSConfig:      tlsServerConfig,
	}

	btl      = &TCPListener{}
	btl2     = &TCPListener{}
	btl3     = &TCPListener{}
	btc      = &TCPClient{}
	btc2     = &TCPClient{}
	name     = "TestMessage"
	date     = time.Now().UnixNano()
	data     = "This is an intenntionally long and rambling sentence to pad out the size of the message."
	msg      = &message.Note{Name: &name, Date: &date, Comment: &data}
	msgBytes = func(*message.Note) []byte { b, _ := proto.Marshal(msg); return b }(msg)
)

func TestMain(m *testing.M) {
	btl, err := ListenTCP(listenConfig)
	if err != nil {
		log.Fatal(err)
	}
	btl.StartListeningAsync()
	defer func() { btl.Close() }()

	btl2, err = ListenTCP(listenConfig2)
	if err != nil {
		log.Fatal(err)
	}
	btl2.StartListeningAsync()
	defer func() { btl2.Close() }()

	btl3, err = ListenTCP(listenConfig3)
	if err != nil {
		log.Fatal(err)
	}
	btl3.StartListeningAsync()
	defer func() { btl3.Close() }()

	btc, err = DialTCP(&buffWriteConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { btc.Close() }()

	btc2, err = DialTCP(&buffWriteConfig2)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { btc2.Close() }()

	os.Exit(m.Run())
}

func TestDialBuffTCPUsesDefaultMessageSize(t *testing.T) {
	cfg := TCPClientConfig{
		Address:   buffWriteConfig.Address,
		TLSConfig: tlsClientConfig,
	}
	buffM, err := DialTCP(&cfg)
	if err != nil {
		t.Errorf("Failed to open connection to %s: %s", cfg.Address, err)
	}
	if buffM.MaxMessageSize != DefaultMaxMessageSize {
		t.Errorf("Expected Max Message Size to be %d, actually got %d", DefaultMaxMessageSize, buffM.MaxMessageSize)
	}
}

func TestDialBuffTCPUsesSpecifiedMaxMessageSize(t *testing.T) {
	cfg := TCPClientConfig{
		Address:        buffWriteConfig.Address,
		MaxMessageSize: 8196,
		TLSConfig:      tlsClientConfig,
	}
	conn, err := DialTCP(&cfg)
	if err != nil {
		t.Errorf("Failed to open connection to %s: %s", cfg.Address, err)
	}
	if conn.MaxMessageSize != cfg.MaxMessageSize {
		t.Errorf("Expected Max Message Size to be %d, actually got %d", cfg.MaxMessageSize, conn.MaxMessageSize)
	}
}

func TestDialTCPUntilConnected(t *testing.T) {
	cfg := TCPClientConfig{
		Address:        FormatAddress("127.0.0.1", strconv.Itoa(5036)),
		MaxMessageSize: 8196,
		TLSConfig:      tlsClientConfig,
	}
	result := make(chan bool)

	go func() {
		_, err := DialTCPUntilConnected(&cfg, time.Second)
		result <- (err == nil)
	}()

	time.Sleep(100 * time.Millisecond)

	serverConfig := TCPListenerConfig{
		MaxMessageSize: 2048,
		Logger:         log.New(os.Stdout, "leanstreams", log.LstdFlags),
		Address:        FormatAddress("", strconv.Itoa(5036)),
		Callback:       exampleCallback,
		TLSConfig:      tlsServerConfig,
	}
	server, err := ListenTCP(serverConfig)
	if err != nil {
		log.Fatal(err)
	}
	server.StartListeningAsync()
	defer func() { server.Close() }()

	successfulConnection := <-result
	if !successfulConnection {
		t.Errorf("Failed to eventually open connection")
	}
}

func TestDialTCPUntilConnectedTimeout(t *testing.T) {
	cfg := TCPClientConfig{
		Address:        FormatAddress("127.0.0.1", strconv.Itoa(5037)),
		MaxMessageSize: 8196,
		TLSConfig:      tlsClientConfig,
	}
	result := make(chan bool)

	go func() {
		_, err := DialTCPUntilConnected(&cfg, 100*time.Millisecond)
		result <- (err == nil)
	}()

	time.Sleep(200 * time.Millisecond)

	serverConfig := TCPListenerConfig{
		MaxMessageSize: 2048,
		Logger:         log.New(os.Stdout, "leanstreams", log.LstdFlags),
		Address:        FormatAddress("", strconv.Itoa(5037)),
		Callback:       exampleCallback,
		TLSConfig:      tlsServerConfig,
	}
	server, err := ListenTCP(serverConfig)
	if err != nil {
		log.Fatal(err)
	}
	server.StartListeningAsync()
	defer func() { server.Close() }()

	successfulConnection := <-result
	if successfulConnection {
		t.Errorf("Failed to timeout dialing before atually connecting")
	}
}

func BenchmarkWrite(b *testing.B) {
	for n := 0; n < b.N; n++ {
		btc.Write(msgBytes)
	}
}

func BenchmarkWrite2(b *testing.B) {
	for n := 0; n < b.N; n++ {
		btc2.Write(msgBytes)
	}
}
