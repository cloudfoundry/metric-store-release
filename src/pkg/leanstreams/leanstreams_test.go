package leanstreams_test

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"strconv"
	"sync"
	"time"

	. "github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams"
	"github.com/cloudfoundry/metric-store-release/src/pkg/leanstreams/test/message"
	"github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	"github.com/cloudfoundry/metric-store-release/src/pkg/tls"
	"github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type leanstreamsTestContext struct {
	Listener   *TCPListener
	Connection *TCPConn

	MessageCommentsReceived []string
	sync.Mutex
}

func (tc *leanstreamsTestContext) Write(comment string) (int, error) {
	name := "Test Message"
	date := time.Now().UnixNano()
	msg := &message.Note{
		Name:    &name,
		Date:    &date,
		Comment: &comment,
	}
	messageBytes, _ := proto.Marshal(msg)

	return tc.Connection.Write(messageBytes)
}

func (tc *leanstreamsTestContext) WaitForResults() {
	Eventually(func() bool {
		if len(tc.Results()) == 1 {
			return true
		}

		time.Sleep(100 * time.Millisecond)
		return false
	}, 1).Should(BeTrue())
}

func (tc *leanstreamsTestContext) Callback(data []byte) error {
	tc.Lock()

	msg := &message.Note{}
	proto.Unmarshal(data, msg)
	comment := *msg.Comment

	tc.MessageCommentsReceived = append(tc.MessageCommentsReceived, comment)

	tc.Unlock()
	return nil
}

func (tc *leanstreamsTestContext) Results() []string {
	tc.Lock()
	defer tc.Unlock()

	return tc.MessageCommentsReceived
}

var _ = Describe("Leanstreams", func() {
	var setup = func(port int) (tc *leanstreamsTestContext, cleanup func()) {
		tc = &leanstreamsTestContext{}

		tlsConfig, err := tls.NewMutualTLSConfig(
			testing.Cert("metric-store-ca.crt"),
			testing.Cert("metric-store.crt"),
			testing.Cert("metric-store.key"),
			"metric-store",
		)
		Expect(err).ToNot(HaveOccurred())

		maxMessageSize := 100
		listenConfig := TCPListenerConfig{
			MaxMessageSize: maxMessageSize,
			EnableLogging:  true,
			Address:        FormatAddress("", strconv.Itoa(port)),
			Callback:       tc.Callback,
			TLSConfig:      tlsConfig,
		}
		listener, err := ListenTCP(listenConfig)
		if err != nil {
			log.Fatal(err)
		}
		listener.StartListeningAsync()
		tc.Listener = listener

		writeConfig := TCPConnConfig{
			MaxMessageSize: maxMessageSize,
			Address:        FormatAddress("127.0.0.1", strconv.Itoa(port)),
			TLSConfig:      tlsConfig,
		}
		connection, err := DialTCP(&writeConfig)
		if err != nil {
			log.Fatal(err)
		}
		tc.Connection = connection

		return tc, func() {
			tc.Connection.Close()
		}
	}

	var randStr = func(len int) string {
		buff := make([]byte, len)
		rand.Read(buff)
		str := base64.StdEncoding.EncodeToString(buff)
		// Base 64 can be longer than len
		return str[:len]
	}

	It("Secure writes to a connection are successfully read by the listener", func() {
		tc, cleanup := setup(5036)
		defer cleanup()

		n, err := tc.Write("This is an example message")
		Expect(n).To(Equal(54))
		Expect(err).ToNot(HaveOccurred())
		tc.WaitForResults()

		receivedData := tc.Results()[0]
		Expect(receivedData).To(Equal("This is an example message"))

		_, err = tc.Write("This is an example message")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When the listeners read buffer is overrun", func() {
		It("recovers and continues to write to a connection", func() {
			tc, cleanup := setup(5037)
			defer cleanup()

			n, err := tc.Write(randStr(200))
			Expect(n).To(Equal(229))
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(time.Second)

			n, err = tc.Write("This is an example message")
			Expect(n).To(Equal(54))
			Expect(err).ToNot(HaveOccurred())
			tc.WaitForResults()

			receivedData := tc.Results()[0]
			Expect(receivedData).To(Equal("This is an example message"))
		})
	})

	Context("When the connection is closed", func() {
		It("The server resumes listening and the client reopens when writing", func() {
			tc, cleanup := setup(5038)
			defer cleanup()

			tc.Listener.Close()

			err := tc.Listener.RestartListeningAsync()
			Expect(err).ToNot(HaveOccurred())

			messageSize, err := tc.Write("This is an example message")
			Expect(messageSize).To(Equal(54))
			Expect(err).ToNot(HaveOccurred())
			tc.WaitForResults()

			receivedData := tc.Results()[0]
			Expect(receivedData).To(Equal("This is an example message"))

			_, err = tc.Write("This is an example message")
			Expect(err).ToNot(HaveOccurred())

			tc.Listener.Close()
			err = tc.Listener.RestartListeningAsync()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
