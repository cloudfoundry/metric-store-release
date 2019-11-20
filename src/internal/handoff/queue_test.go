package handoff_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/internal/handoff"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Queue of writes", func() {
	It("appends one entry to the file", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("test"))
		Expect(err).NotTo(HaveOccurred())

		exp := filepath.Join(tc.dir, "1")
		stats, err := os.Stat(exp)
		Expect(err).NotTo(HaveOccurred())

		// 8 byte header ptr + 8 byte record len + record len
		Expect(stats.Size()).To(Equal(int64(8 + 8 + 4)))

		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("test"))
	})

	It("appends multiple entries to the file", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		for _, expectation := range []string{"one", "two"} {
			cur, err := tc.queue.Current()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cur)).To(Equal(expectation))

			err = tc.queue.Advance()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("persists the read footer", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("one"))

		err = tc.queue.Advance()
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Close()
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Open()
		Expect(err).NotTo(HaveOccurred())

		cur, err = tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("two"))
	})

	It("initialize current info when writing the first record", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Advance()
		Expect(err).NotTo(HaveOccurred())

		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("two"))
	})

	It("correctly handles multiple segments", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		// append one entry, should go to the first segment
		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		// set the segment size low to force a new segment to be created
		tc.queue.SetMaxSegmentSize(12)

		// goes into a new segment
		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		// reads from first segment
		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("one"))

		err = tc.queue.Advance()
		Expect(err).NotTo(HaveOccurred())

		// ensure the first segment file is removed since we've advanced past the end
		_, err = os.Stat(filepath.Join(tc.dir, "1"))
		Expect(err).To(HaveOccurred())
		Expect(os.IsNotExist(err)).To(BeTrue())

		// reads from second segment
		cur, err = tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("two"))

		_, err = os.Stat(filepath.Join(tc.dir, "2"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Advance()
		Expect(err).NotTo(HaveOccurred())

		_, err = tc.queue.Current()
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(io.EOF))
	})

	It("returns an error when the queue is full", func() {
		tc, cleanup := setupHandoffTestContext(10)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).To(Equal(handoff.ErrQueueFull))
	})

	It("reopens a closed queue", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("one"))

		// close and re-open the queue
		err = tc.queue.Close()
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Open()
		Expect(err).NotTo(HaveOccurred())

		// Make sure we can read back the last current value
		cur, err = tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("one"))

		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		err = tc.queue.Advance()
		Expect(err).NotTo(HaveOccurred())

		cur, err = tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("two"))
	})

	It("skips opening corrupt segments while still reading from valid segments", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		tc.queue.SetMaxSegmentSize(19)

		err = tc.queue.Append([]byte("two"))
		Expect(err).NotTo(HaveOccurred())

		tc.queue.Close()

		files, err := ioutil.ReadDir(tc.dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(HaveLen(2))

		fileToCorrupt, err := os.OpenFile(filepath.Join(tc.dir, files[0].Name()), os.O_CREATE|os.O_RDWR, 0600)
		Expect(err).NotTo(HaveOccurred())

		_, err = fileToCorrupt.Seek(-8, os.SEEK_END)
		Expect(err).NotTo(HaveOccurred())

		corruptBytes := []byte{99, 99, 99, 99, 99, 99, 99, 99}
		numberOfBytes, err := fileToCorrupt.Write(corruptBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(numberOfBytes).To(Equal(8))

		fileToCorrupt.Sync()
		fileToCorrupt.Close()

		q, err := handoff.NewQueue(tc.dir, 1024)
		Expect(err).NotTo(HaveOccurred())

		err = q.Open()
		Expect(err).NotTo(HaveOccurred())

		cur, err := q.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("two"))
	})

	It("purges a queue", func() {
		tc, cleanup := setupHandoffTestContext(1024)
		defer cleanup()

		err := tc.queue.Append([]byte("one"))
		Expect(err).NotTo(HaveOccurred())

		cur, err := tc.queue.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cur)).To(Equal("one"))

		time.Sleep(time.Second)

		err = tc.queue.PurgeOlderThan(time.Now())
		Expect(err).NotTo(HaveOccurred())

		_, err = tc.queue.Current()
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(io.EOF))
	})
})

type handoffTestContext struct {
	dir   string
	queue *handoff.Queue
}

func setupHandoffTestContext(queueSize int64) (*handoffTestContext, func()) {
	dir, err := ioutil.TempDir("", "hh_queue")
	Expect(err).NotTo(HaveOccurred())

	q, err := handoff.NewQueue(dir, queueSize)
	Expect(err).NotTo(HaveOccurred())

	err = q.Open()
	Expect(err).NotTo(HaveOccurred())

	tc := &handoffTestContext{
		dir:   dir,
		queue: q,
	}

	return tc, func() {
		os.RemoveAll(dir)
	}
}
