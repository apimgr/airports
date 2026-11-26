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
	Version          string       `json:"version"`
	CreatedAt        string       `json:"created_at"`
	CreatedBy        string       `json:"created_by"`
	AppVersion       string       `json:"app_version"`
	Contents         []string     `json:"contents"`
	Encrypted        bool         `json:"encrypted"`
	EncryptionMethod string       `json:"encryption_method,omitempty"`
	Type             string       `json:"type"`
	SnapshotAt       string       `json:"snapshot_at"`
	Files            []BackupFile `json:"files"`
	BaseBackup       string       `json:"base_backup,omitempty"`
	BaseSHA256       string       `json:"base_sha256,omitempty"`
	Deleted          []string     `json:"deleted,omitempty"`
	Checksum         string       `json:"sha256"`
	legacy           bool
}

// BackupFile is a content file recorded by the canonical backup manifest.
type BackupFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	MTime  int64  `json:"mtime"`
	SHA256 string `json:"sha256"`
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
	if m.Type == "" && m.SnapshotAt == "" {
		var legacy struct {
			Checksum string `json:"checksum"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return nil, fmt.Errorf("backup: parse legacy manifest: %w", err)
		}
		if legacy.Checksum == "" || m.CreatedAt == "" {
			return nil, fmt.Errorf("backup: legacy manifest is missing checksum or created_at")
		}
		m.Type = "full"
		m.SnapshotAt = m.CreatedAt
		m.Checksum = legacy.Checksum
		m.legacy = true
		return &m, nil
	}
	if m.Type != "full" && m.Type != "incremental" {
		return nil, fmt.Errorf("backup: invalid manifest type %q", m.Type)
	}
	if m.SnapshotAt == "" || m.Checksum == "" {
		return nil, fmt.Errorf("backup: manifest is missing snapshot_at or sha256")
	}
	if m.Type == "incremental" && (m.BaseBackup == "" || m.BaseSHA256 == "") {
		return nil, fmt.Errorf("backup: incremental manifest is missing base metadata")
	}
	return &m, nil
}
