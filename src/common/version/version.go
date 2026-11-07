// Package version contains build metadata helpers shared by the server and
// command-line client binaries.
package version

import (
	"fmt"
	"strconv"
	"time"
)

// BuildInfo contains the values embedded by the linker or derived at startup.
type BuildInfo struct {
	Version      string
	CommitID     string
	BuildDate    string
	BuildEpoch   string
	OfficialSite string
}

// ParseBuildEpoch parses a Unix timestamp in seconds, returning zero when it
// is empty, invalid, or outside the representable range.
func ParseBuildEpoch(raw string) int64 {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || epoch <= 0 {
		return 0
	}
	return epoch
}

// BuildDateFromEpoch formats a valid Unix timestamp as RFC 3339 UTC. If the
// timestamp is unset or invalid, it returns fallback unchanged.
func BuildDateFromEpoch(raw, fallback string) string {
	epoch := ParseBuildEpoch(raw)
	if epoch == 0 {
		return fallback
	}
	return time.Unix(epoch, 0).UTC().Format(time.RFC3339)
}

// Format returns the established one-line version display for a binary.
func (info BuildInfo) Format(binaryName string) string {
	return fmt.Sprintf("%s %s (commit %s, built %s)", binaryName, info.Version, info.CommitID, info.BuildDate)
}
