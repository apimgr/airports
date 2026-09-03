package backup

import (
	"os"
	"testing"
)

// testTempDir mirrors src/ssl/ssl_test.go's testTempDir helper, per
// testing-rules.md's temp-dir convention: all test/runtime data under
// /tmp/apimgr/airports-XXXXXX/, never bare /tmp.
func testTempDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/tmp/apimgr", 0o755); err != nil {
		t.Fatalf("MkdirAll base: %v", err)
	}
	dir, err := os.MkdirTemp("/tmp/apimgr", "airports-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}
