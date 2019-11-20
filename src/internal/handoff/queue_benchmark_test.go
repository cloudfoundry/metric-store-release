package handoff_test

import (
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
)

var _ = Describe("Handoff", func() {
	It("", func() {
	})
})

// TODO: keep as benchmark test
// func BenchmarkQueueAppend(b *testing.B) {
// 	dir, err := ioutil.TempDir("", "hh_queue")
// 	if err != nil {
// 		b.Fatalf("failed to create temp dir: %v", err)
// 	}
// 	defer os.RemoveAll(dir)

// 	q, err := handoff.NewQueue(dir, 1024*1024*1024)
// 	if err != nil {
// 		b.Fatalf("failed to create queue: %v", err)
// 	}

// 	if err := q.Open(); err != nil {
// 		b.Fatalf("failed to open queue: %v", err)
// 	}

// 	for i := 0; i < b.N; i++ {
// 		if err := q.Append([]byte(fmt.Sprintf("%d", i))); err != nil {
// 			println(q.DiskUsage())
// 			b.Fatalf("Queue.Append failed: %v", err)
// 		}
// 	}
// }
