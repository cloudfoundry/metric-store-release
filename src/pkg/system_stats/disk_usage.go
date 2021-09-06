package system_stats

import (
	"golang.org/x/sys/unix"

	"github.com/cloudfoundry/metric-store-release/src/internal/metrics"
	"github.com/cloudfoundry/metric-store-release/src/pkg/logger"
	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
)

func DiskFree(path string) (float64, error) {
	var fsInfo unix.Statfs_t
	err := unix.Statfs(path, &fsInfo)
	if err != nil {
		return 0, err
	}
	if fsInfo.Blocks == 0 {
		return 0, nil
	}
	// This is specifically the blocks available to unprivileged users (as
	// we will not be running as root). It will be lower than `df`.
	return 100 * (float64(fsInfo.Bavail) / float64(fsInfo.Blocks)), nil
}

func NewDiskFreeReporter(storagePath string, log *logger.Logger, registrar metrics.Registrar) func() (float64, error) {
	return func() (float64, error) {
		diskFree, err := DiskFree(storagePath)

		if err != nil {
			log.Error("failed to get disk free space", err, logger.String("path", storagePath))
			return persistence.UNKNOWN_DISK_FREE_PERCENT, err
		}

		registrar.Set(metrics.MetricStoreDiskFreeRatio, diskFree/100)
		return diskFree, nil
	}
}
