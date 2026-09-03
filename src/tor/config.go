package tor

// Config holds all Tor-related settings sourced from the operator's
// server.yml `server.tor` section per AI.md PART 31 "Configuration".
// Enabled is a tri-state pointer: nil means "not set in server.yml", which
// resolves to auto-detect (enabled only if a tor binary is found on the
// system); an explicit true/false always overrides auto-detection.
type Config struct {
	// Enabled overrides auto-detection when non-nil. Per PART 31, the
	// hidden service is normally always-on when a tor binary is found, but
	// this project's config schema still exposes an explicit override so
	// an operator can force Tor off even when a binary is present.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Binary is the path to the tor executable. Empty means auto-detect
	// via PATH and common per-OS install locations.
	Binary string `yaml:"binary" json:"binary"`

	// UseNetwork routes the server's own outbound HTTP requests through
	// the dedicated Tor process's SOCKS proxy. Separate from hosting the
	// hidden service.
	UseNetwork bool `yaml:"use_network" json:"use_network"`

	// MaxCircuits caps the number of circuits Tor keeps open (1-128).
	MaxCircuits int `yaml:"max_circuits" json:"max_circuits"`

	// CircuitTimeout is how long, in seconds, Tor waits before giving up
	// on a circuit (10-300).
	CircuitTimeout int `yaml:"circuit_timeout" json:"circuit_timeout"`

	// BootstrapTimeout is how long, in seconds, we wait for Tor to finish
	// connecting to the Tor network on startup (30-600).
	BootstrapTimeout int `yaml:"bootstrap_timeout" json:"bootstrap_timeout"`

	// SafeLogging scrubs sensitive information from Tor's own log output.
	SafeLogging bool `yaml:"safe_logging" json:"safe_logging"`

	// MaxStreamsPerCircuit caps concurrent streams per circuit (10-500).
	MaxStreamsPerCircuit int `yaml:"max_streams_per_circuit" json:"max_streams_per_circuit"`

	// CloseCircuitOnStreamLimit closes a circuit once MaxStreamsPerCircuit
	// is exceeded rather than queueing further streams on it.
	CloseCircuitOnStreamLimit bool `yaml:"close_circuit_on_stream_limit" json:"close_circuit_on_stream_limit"`

	// BandwidthRate is Tor's sustained bandwidth cap, e.g. "1 MB".
	BandwidthRate string `yaml:"bandwidth_rate" json:"bandwidth_rate"`

	// BandwidthBurst is Tor's burst bandwidth cap, e.g. "2 MB".
	BandwidthBurst string `yaml:"bandwidth_burst" json:"bandwidth_burst"`

	// MaxMonthlyBandwidth is the monthly accounting cap, e.g. "100 GB", or
	// the literal string "unlimited" to disable accounting entirely.
	MaxMonthlyBandwidth string `yaml:"max_monthly_bandwidth" json:"max_monthly_bandwidth"`

	// NumIntroPoints is the number of hidden-service introduction points
	// (3-10). Higher is more resilient but generates more background
	// traffic.
	NumIntroPoints int `yaml:"num_intro_points" json:"num_intro_points"`

	// VirtualPort is the port external Tor clients connect to on the
	// .onion address (typically 80).
	VirtualPort int `yaml:"virtual_port" json:"virtual_port"`
}

// DefaultConfig returns the documented PART 31 defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:                   nil,
		Binary:                    "",
		UseNetwork:                false,
		MaxCircuits:               32,
		CircuitTimeout:            60,
		BootstrapTimeout:          180,
		SafeLogging:               true,
		MaxStreamsPerCircuit:      100,
		CloseCircuitOnStreamLimit: true,
		BandwidthRate:             "1 MB",
		BandwidthBurst:            "2 MB",
		MaxMonthlyBandwidth:       "100 GB",
		NumIntroPoints:            3,
		VirtualPort:               80,
	}
}

// applyDefaults fills any zero-value numeric/string fields with the PART 31
// documented defaults, leaving explicitly-set operator values untouched.
// This keeps a partially-specified server.yml `tor:` block safe to use.
func applyDefaults(cfg Config) Config {
	d := DefaultConfig()

	if cfg.MaxCircuits <= 0 {
		cfg.MaxCircuits = d.MaxCircuits
	}
	if cfg.CircuitTimeout <= 0 {
		cfg.CircuitTimeout = d.CircuitTimeout
	}
	if cfg.BootstrapTimeout <= 0 {
		cfg.BootstrapTimeout = d.BootstrapTimeout
	}
	if cfg.MaxStreamsPerCircuit <= 0 {
		cfg.MaxStreamsPerCircuit = d.MaxStreamsPerCircuit
	}
	if cfg.BandwidthRate == "" {
		cfg.BandwidthRate = d.BandwidthRate
	}
	if cfg.BandwidthBurst == "" {
		cfg.BandwidthBurst = d.BandwidthBurst
	}
	if cfg.MaxMonthlyBandwidth == "" {
		cfg.MaxMonthlyBandwidth = d.MaxMonthlyBandwidth
	}
	if cfg.NumIntroPoints <= 0 {
		cfg.NumIntroPoints = d.NumIntroPoints
	}
	if cfg.VirtualPort <= 0 {
		cfg.VirtualPort = d.VirtualPort
	}

	return cfg
}

// resolveEnabled implements the PART 31 / task-defined auto-enable rule:
// an explicit Enabled value always wins; otherwise Tor is considered
// enabled only when a usable tor binary can be located.
func resolveEnabled(cfg Config, findBinary func(configured string) (string, error)) bool {
	if cfg.Enabled != nil {
		return *cfg.Enabled
	}
	_, err := findBinary(cfg.Binary)
	return err == nil
}
