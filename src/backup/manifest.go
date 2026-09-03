package backup

import (
	"encoding/json"
	"fmt"
)

// ManifestVersion is the manifest schema version, independent of app_version.
const ManifestVersion = "1.0.0"

// Manifest describes a backup archive's contents and provenance, per AI.md
// PART 21 "Backup Format" -> manifest.json.
type Manifest struct {
	Version          string   `json:"version"`
	CreatedAt        string   `json:"created_at"`
	CreatedBy        string   `json:"created_by"`
	AppVersion       string   `json:"app_version"`
	Contents         []string `json:"contents"`
	Encrypted        bool     `json:"encrypted"`
	EncryptionMethod string   `json:"encryption_method,omitempty"`
	Checksum         string   `json:"checksum"`
}

// Marshal serializes the manifest to indented JSON.
func (m *Manifest) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("backup: marshal manifest: %w", err)
	}
	return data, nil
}

// UnmarshalManifest parses manifest JSON.
func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("backup: parse manifest: %w", err)
	}
	return &m, nil
}
