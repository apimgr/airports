//go:build !windows

package backup

import "golang.org/x/sys/unix"

// DiskUsage returns the total and free bytes for the filesystem containing
// path, used by preflightDiskSpace and the "%"-based max_total_size parser.
func DiskUsage(path string) (total, free uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bavail * uint64(stat.Bsize)
	return total, free, nil
}
