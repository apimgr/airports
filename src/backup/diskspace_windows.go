//go:build windows

package backup

import "golang.org/x/sys/windows"

// DiskUsage returns the total and free bytes for the volume containing
// path, used by preflightDiskSpace and the "%"-based max_total_size parser.
func DiskUsage(path string) (total, free uint64, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, 0, err
	}
	return totalBytes, freeBytesAvailable, nil
}
