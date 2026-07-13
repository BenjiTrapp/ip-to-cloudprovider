package shodan

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// apiKeyEnv is the environment variable checked for a Shodan key when the
// config file omits it.
const apiKeyEnv = "SHODAN_API_KEY"

// Config holds the Shodan settings. It lives under the `shodan:` key of the
// shared config file (the same file the reputation sources use), so a single
// file configures everything.
type Config struct {
	Shodan Settings `yaml:"shodan"`
}

// Settings configures the Shodan client.
type Settings struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
}

// applyDefaults pulls the API key from the environment when the file omits it.
func (c *Config) applyDefaults() {
	if c.Shodan.APIKey == "" {
		if key := os.Getenv(apiKeyEnv); key != "" {
			c.Shodan.APIKey = key
		}
	}
}

// APIKey returns the configured Shodan API key (empty if none is set).
func (c Config) APIKey() string { return c.Shodan.APIKey }

// DefaultConfig returns a Config with only environment-derived values applied.
func DefaultConfig() Config {
	cfg := Config{}
	cfg.applyDefaults()
	return cfg
}

// LoadConfig reads the Shodan config from path. When path is empty the default
// location is used. A missing file is not an error: the key may still come from
// the SHODAN_API_KEY environment variable.
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

// DefaultConfigPath returns the shared config file location, matching the
// reputation package so a single file holds all settings. It honors the
// IP2CP_REPUTATION_CONFIG override and XDG conventions.
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
