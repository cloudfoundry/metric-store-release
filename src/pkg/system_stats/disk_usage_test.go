package system_stats_test

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/cloudfoundry/metric-store-release/src/pkg/system_stats"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Disk Usage Reporter", func() {
	It("returns the same value as df", func() {
		path := newTemporaryDir()

		diskFree, err := system_stats.DiskFree(path)
		Expect(err).ToNot(HaveOccurred())

		Expect(diskFree).To(BeNumerically("~", getDiskFreeFromDf(path), .1))

		err = os.RemoveAll(path)
		Expect(err).ToNot(HaveOccurred())
	})

	It("passes through errors", func() {
		_, err := system_stats.DiskFree("")

		Expect(err).To(HaveOccurred())
	})
})

func newTemporaryDir() string {
	path, err := ioutil.TempDir(os.Getenv("TEMPDIR"), "metric-store-disk-usage-test")
	Expect(err).ToNot(HaveOccurred())

	return path
}

var whitespaceMatcher = regexp.MustCompile(`\s+`)

func getDiskFreeFromDf(path string) float64 {
	cmd := exec.Command("df", "--portability", path)
	dfStdout, err := cmd.StdoutPipe()
	Expect(err).ToNot(HaveOccurred())
	defer dfStdout.Close()

	err = cmd.Start()
	Expect(err).ToNot(HaveOccurred())

	dfStdoutReader := bufio.NewReader(dfStdout)
	// discard header
	_, err = dfStdoutReader.ReadString('\n')
	Expect(err).ToNot(HaveOccurred())

	output, err := dfStdoutReader.ReadString('\n')

	columns := whitespaceMatcher.Split(output, -1)
	Expect(columns).To(HaveLen(7))

	totalBlocks, err := strconv.ParseFloat(columns[1], 64)
	Expect(err).ToNot(HaveOccurred())

	availableBlocks, err := strconv.ParseFloat(columns[3], 64)
	Expect(err).ToNot(HaveOccurred())

	err = cmd.Wait()
	Expect(err).ToNot(HaveOccurred())

	return 100 * (availableBlocks / totalBlocks)
}
