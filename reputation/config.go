package reputation

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config controls which reputation sources are active and how they are
// configured. It is designed to work with zero configuration: with an empty
// Config, DefaultConfig fills in the keyless DNSBL sources so a scan works out
// of the box without any API key or registration.
type Config struct {
	// DNSBLs lists DNS blocklist sources. When nil, the built-in defaults are used.
	DNSBLs []DNSBLConfig `yaml:"dnsbls"`

	// AbuseIPDB configures the AbuseIPDB API source (opt-in, requires a key).
	AbuseIPDB AbuseIPDBConfig `yaml:"abuseipdb"`
}

// DNSBLConfig describes a single DNS blocklist source.
type DNSBLConfig struct {
	Name    string `yaml:"name"`
	Zone    string `yaml:"zone"`
	Score   int    `yaml:"score"`
	Enabled *bool  `yaml:"enabled"` // pointer so "unset" differs from "false"
}

// AbuseIPDBConfig configures the AbuseIPDB source.
type AbuseIPDBConfig struct {
	Enabled   bool   `yaml:"enabled"`
	APIKey    string `yaml:"api_key"`
	MaxAgeDay int    `yaml:"max_age_days"`
}

// abuseIPDBKeyEnv is the environment variable checked for an AbuseIPDB key
// when none is set in the config file.
const abuseIPDBKeyEnv = "ABUSEIPDB_API_KEY"

// defaultDNSBLs are the keyless DNS blocklists enabled out of the box.
// Spamhaus ZEN is the combined Spamhaus list; the others add breadth.
var defaultDNSBLs = []DNSBLConfig{
	{Name: "spamhaus-zen", Zone: "zen.spamhaus.org", Score: 80},
	{Name: "spamcop", Zone: "bl.spamcop.net", Score: 60},
	{Name: "barracuda", Zone: "b.barracudacentral.org", Score: 60},
	{Name: "uceprotect-l1", Zone: "dnsbl-1.uceprotect.net", Score: 50},
}

// DefaultConfig returns a Config with the keyless DNSBL sources enabled and
// AbuseIPDB disabled (it requires a key). The AbuseIPDB key is still picked up
// from the environment so users can enable it purely via env + config toggle.
func DefaultConfig() Config {
	cfg := Config{}
	cfg.applyDefaults()
	return cfg
}

// applyDefaults fills in built-in DNSBLs when none are configured and pulls the
// AbuseIPDB key from the environment when the config omits it.
func (c *Config) applyDefaults() {
	if c.DNSBLs == nil {
		c.DNSBLs = append([]DNSBLConfig(nil), defaultDNSBLs...)
	}
	if c.AbuseIPDB.APIKey == "" {
		if key := os.Getenv(abuseIPDBKeyEnv); key != "" {
			c.AbuseIPDB.APIKey = key
		}
	}
}

// sources builds the ordered list of active Source implementations.
func (c Config) sources() []Source {
	var sources []Source

	for _, d := range c.DNSBLs {
		if d.Enabled != nil && !*d.Enabled {
			continue
		}
		if d.Zone == "" {
			continue
		}
		score := d.Score
		if score <= 0 {
			score = suspiciousThreshold
		}
		name := d.Name
		if name == "" {
			name = d.Zone
		}
		sources = append(sources, NewDNSBL(name, d.Zone, score))
	}

	if c.AbuseIPDB.Enabled && c.AbuseIPDB.APIKey != "" {
		sources = append(sources, NewAbuseIPDB(c.AbuseIPDB.APIKey, c.AbuseIPDB.MaxAgeDay))
	}

	return sources
}

// LoadConfig reads a reputation config from the given path. When path is empty,
// the default location is used. A missing file is not an error: the built-in
// defaults are returned so scans work without any setup.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// DefaultConfigPath returns the default reputation config file location,
// honoring the IP2CP_REPUTATION_CONFIG override and XDG conventions.
func DefaultConfigPath() string {
	if p := os.Getenv("IP2CP_REPUTATION_CONFIG"); p != "" {
		return p
	}

	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "ip-to-cloudprovider", "reputation.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "reputation.yaml"
	}
	return filepath.Join(home, ".config", "ip-to-cloudprovider", "reputation.yaml")
}
