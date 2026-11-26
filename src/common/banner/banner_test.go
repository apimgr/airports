package banner

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintStartupBannerIncludesApplicationName(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	PrintStartupBanner(BannerConfig{AppName: "Airports Client", Version: "1.2.3"})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Airports Client") {
		t.Fatalf("startup banner omitted application name: %q", output.String())
	}
}
