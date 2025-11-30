package data

import "embed"

//go:embed *.json
var EmbeddedData embed.FS

// ReadFile reads an embedded data file
func ReadFile(name string) ([]byte, error) {
	return EmbeddedData.ReadFile(name)
}
