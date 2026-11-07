package i2p

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Config controls the optional I2P eepsite.
type Config struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	Binary           string `yaml:"binary" json:"binary"`
	SAMAddress       string `yaml:"sam_address" json:"sam_address"`
	VirtualPort      int    `yaml:"virtual_port" json:"virtual_port"`
	InboundLength    int    `yaml:"inbound_length" json:"inbound_length"`
	OutboundLength   int    `yaml:"outbound_length" json:"outbound_length"`
	InboundQuantity  int    `yaml:"inbound_quantity" json:"inbound_quantity"`
	OutboundQuantity int    `yaml:"outbound_quantity" json:"outbound_quantity"`
	SignatureType    int    `yaml:"signature_type" json:"signature_type"`
	BootstrapTimeout string `yaml:"bootstrap_timeout" json:"bootstrap_timeout"`
}

// DefaultConfig returns opt-in-off defaults for the I2P eepsite.
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		SAMAddress:       "127.0.0.1:7656",
		VirtualPort:      80,
		InboundLength:    3,
		OutboundLength:   3,
		InboundQuantity:  5,
		OutboundQuantity: 5,
		SignatureType:    7,
		BootstrapTimeout: "5m",
	}
}

// WithDefaults fills omitted fields without enabling I2P.
func (c Config) WithDefaults() Config {
	d := DefaultConfig()
	if c.SAMAddress == "" {
		c.SAMAddress = d.SAMAddress
	}
	if strings.TrimSpace(c.BootstrapTimeout) == "" {
		c.BootstrapTimeout = d.BootstrapTimeout
	}
	return c
}

// Validate checks the documented I2P tunnel ranges and timeout.
func (c Config) Validate() error {
	c = c.WithDefaults()
	if c.VirtualPort < 1 || c.VirtualPort > 65535 {
		return fmt.Errorf("server.i2p.virtual_port must be between 1 and 65535")
	}
	if c.InboundLength < 0 || c.InboundLength > 7 || c.OutboundLength < 0 || c.OutboundLength > 7 {
		return fmt.Errorf("server.i2p inbound_length and outbound_length must be between 0 and 7")
	}
	if c.InboundQuantity < 1 || c.InboundQuantity > 16 || c.OutboundQuantity < 1 || c.OutboundQuantity > 16 {
		return fmt.Errorf("server.i2p inbound_quantity and outbound_quantity must be between 1 and 16")
	}
	if c.SignatureType < 0 || c.SignatureType > 255 {
		return fmt.Errorf("server.i2p.signature_type must be between 0 and 255")
	}
	duration, err := time.ParseDuration(c.BootstrapTimeout)
	if err != nil || duration < 30*time.Second || duration > 10*time.Minute {
		return fmt.Errorf("server.i2p.bootstrap_timeout must be between 30s and 10m")
	}
	if c.SAMAddress != "" {
		host, port, err := net.SplitHostPort(c.SAMAddress)
		if err != nil || strings.TrimSpace(host) == "" {
			return fmt.Errorf("server.i2p.sam_address must be a host:port address")
		}
		portNumber, parseErr := strconv.Atoi(port)
		if parseErr != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("server.i2p.sam_address port must be between 1 and 65535")
		}
	}
	return nil
}
