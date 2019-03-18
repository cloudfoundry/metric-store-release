package system_stats

import (
	"golang.org/x/sys/unix"
)

func DiskFree(path string) (float64, error) {
	var fsInfo unix.Statfs_t
	err := unix.Statfs(path, &fsInfo)
	if err != nil {
		return 0, err
	}

	// This is specifically the blocks available to unprivileged users (as
	// we will not be running as root). It will be lower than `df`.
	return 100 * (float64(fsInfo.Bavail) / float64(fsInfo.Blocks)), nil
}
