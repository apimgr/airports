package config

// AllowlistEntry is a single trusted IP/CIDR entry from
// server.security.allowlist, per AI.md PART 11 "IP Block Management".
// Single IPs auto-expand to /32 (IPv4) or /128 (IPv6); overly broad ranges
// (IPv4 /0-/7, IPv6 /0-/15) are rejected at parse time.
type AllowlistEntry struct {
	CIDR        string `yaml:"cidr" json:"cidr"`               // IP or CIDR, e.g. "192.168.1.0/24"
	Description string `yaml:"description" json:"description"` // Human-readable label (required for clarity)
}

// BlockedIPEntry is a single permanent IP block from
// server.security.blocked_ips, per AI.md PART 11 "IP Block Management".
// Permanent blocks are config-file only — released by editing the config,
// never automatically.
type BlockedIPEntry struct {
	CIDR   string `yaml:"cidr" json:"cidr"`     // IP or CIDR to block
	Reason string `yaml:"reason" json:"reason"` // Human-readable block reason
}
