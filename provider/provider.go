// Package provider defines the cloud provider registry and common utilities
// for fetching, parsing, storing, and querying IP range data.
package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EmbeddedData holds a build-time snapshot of provider IP ranges, laid out as
// "<provider>/ipranges.json". It is populated by the main package via go:embed
// and used as a read-only fallback when no data exists on disk. A nil value
// disables the fallback (e.g. in tests that assert the no-data path).
var EmbeddedData fs.FS

// embeddedRange reads a provider's IP ranges from the embedded snapshot.
// Returns nil if no embedded data is available for the provider.
func embeddedRange(providerName string) (*IPRange, error) {
	if EmbeddedData == nil {
		return nil, fmt.Errorf("no embedded data")
	}
	data, err := fs.ReadFile(EmbeddedData, providerName+"/ipranges.json")
	if err != nil {
		return nil, err
	}
	var ipRange IPRange
	if err := json.Unmarshal(data, &ipRange); err != nil {
		return nil, fmt.Errorf("unmarshalling embedded %s: %w", providerName, err)
	}
	return &ipRange, nil
}

// hasEmbedded reports whether the embedded snapshot contains data for a provider.
func hasEmbedded(providerName string) bool {
	if EmbeddedData == nil {
		return false
	}
	_, err := fs.Stat(EmbeddedData, providerName+"/ipranges.json")
	return err == nil
}

const (
	// httpTimeout is the maximum time allowed for a single HTTP request.
	httpTimeout = 30 * time.Second

	// maxResponseSize is the maximum response body size (50 MB).
	maxResponseSize = 50 * 1024 * 1024
)

// httpClient is a shared HTTP client with sensible timeouts.
var httpClient = &http.Client{
	Timeout: httpTimeout,
}

// IPRange holds IPv4 and IPv6 CIDR ranges for a provider.
type IPRange struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// ParseFunc parses raw response bytes into an IPRange.
type ParseFunc func(data []byte) (*IPRange, error)

// UpdateFunc is an alternative update strategy for providers that require
// multi-step fetching (e.g. Microsoft). It writes data directly.
type UpdateFunc func(dataDir string) error

// Provider represents a cloud provider with its metadata and parsing logic.
type Provider struct {
	Name   string
	URL    string
	Parse  ParseFunc
	Update UpdateFunc // if set, used instead of URL+Parse
}

// Registry holds all registered providers in order.
var Registry []Provider

// Register adds a provider to the global registry.
func Register(p Provider) {
	Registry = append(Registry, p)
}

// ByName returns a provider by name, or nil if not found.
func ByName(name string) *Provider {
	for i := range Registry {
		if Registry[i].Name == name {
			return &Registry[i]
		}
	}
	return nil
}

// Names returns all registered provider names.
func Names() []string {
	names := make([]string, len(Registry))
	for i, p := range Registry {
		names[i] = p.Name
	}
	return names
}

// userAgent is sent with all outgoing HTTP requests.
const userAgent = "ip-to-cloudprovider/1.0 (https://github.com/BenjiTrapp/ip-to-cloudprovider)"

// Fetch downloads data from a URL with timeout and size limits.
func Fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	// Limit response body to prevent OOM
	limited := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}
	if int64(len(body)) > maxResponseSize {
		return nil, fmt.Errorf("response from %s exceeds %d MB limit", url, maxResponseSize/1024/1024)
	}

	return body, nil
}

// FetchAndParse downloads data from the provider's URL and parses it.
func FetchAndParse(p *Provider) (*IPRange, error) {
	if p.Parse == nil {
		return nil, fmt.Errorf("provider %s has no parser", p.Name)
	}

	body, err := Fetch(p.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", p.Name, err)
	}

	return p.Parse(body)
}

// UpdateProvider fetches and saves the IP ranges for a provider.
// If the provider has a custom Update function, it is used instead.
func UpdateProvider(p *Provider, dataDir string) error {
	if p.Update != nil {
		return p.Update(dataDir)
	}

	ipRange, err := FetchAndParse(p)
	if err != nil {
		return err
	}

	return Save(p.Name, ipRange, dataDir)
}

// Save writes an IPRange to disk as JSON, validating CIDRs before saving.
func Save(providerName string, ipRange *IPRange, dataDir string) error {
	// Validate and filter CIDRs
	validated := &IPRange{
		IPv4: validateCIDRs(ipRange.IPv4),
		IPv6: validateCIDRs(ipRange.IPv6),
	}

	dir := filepath.Join(dataDir, providerName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.Marshal(validated)
	if err != nil {
		return fmt.Errorf("marshalling %s data: %w", providerName, err)
	}

	path := filepath.Join(dir, "ipranges.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// Load reads an IPRange from disk, falling back to the embedded snapshot when
// no data file exists in the data directory.
func Load(providerName, dataDir string) (*IPRange, error) {
	path := filepath.Join(dataDir, providerName, "ipranges.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if ipRange, embErr := embeddedRange(providerName); embErr == nil {
				return ipRange, nil
			}
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var ipRange IPRange
	if err := json.Unmarshal(data, &ipRange); err != nil {
		return nil, fmt.Errorf("unmarshalling %s: %w", path, err)
	}

	return &ipRange, nil
}

// HasData returns true if the given provider has data available, either as a
// file in the data directory or in the embedded snapshot.
func HasData(providerName, dataDir string) bool {
	path := filepath.Join(dataDir, providerName, "ipranges.json")
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return hasEmbedded(providerName)
}

// HasAnyData returns true if at least one provider has data loaded.
func HasAnyData(dataDir string) bool {
	for _, p := range Registry {
		if HasData(p.Name, dataDir) {
			return true
		}
	}
	return false
}

// CheckIP checks if an IP is in the provider's ranges. Returns the provider
// name if found, or an empty string if not.
// NOTE: For batch operations, use Matcher instead (pre-loads and caches data).
func CheckIP(ip, dataDir string) string {
	for _, p := range Registry {
		ipRange, err := Load(p.Name, dataDir)
		if err != nil {
			continue
		}
		if IsIPInRange(ip, ipRange.IPv4) || IsIPInRange(ip, ipRange.IPv6) {
			return p.Name
		}
	}
	return ""
}

// IsIPInRange checks if an IP belongs to any of the given CIDR ranges.
func IsIPInRange(ip string, ranges []string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, cidr := range ranges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// ClassifyCIDR determines whether a CIDR string is IPv4 or IPv6.
func ClassifyCIDR(cidr string) (isIPv6 bool) {
	return strings.Contains(cidr, ":")
}

// ParsePlainTextCIDRs parses a plain text list of CIDRs (one per line).
// This is the common format used by Google, OpenAI, and similar providers.
func ParsePlainTextCIDRs(data []byte) (*IPRange, error) {
	lines := strings.Split(string(data), "\n")

	ipRange := &IPRange{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if ClassifyCIDR(line) {
			ipRange.IPv6 = append(ipRange.IPv6, line)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, line)
		}
	}

	return ipRange, nil
}

// validateCIDRs filters a list of CIDRs, keeping only valid ones.
func validateCIDRs(cidrs []string) []string {
	if cidrs == nil {
		return nil
	}
	valid := make([]string, 0, len(cidrs))
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if _, _, err := net.ParseCIDR(cidr); err == nil {
			valid = append(valid, cidr)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	return valid
}

// DefaultDataDir returns the default data directory based on OS conventions.
// Uses XDG_DATA_HOME on Linux, %LOCALAPPDATA% on Windows, ~/Library on macOS.
func DefaultDataDir() string {
	// Check environment override
	if dir := os.Getenv("IP2CP_DATA_DIR"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	switch {
	case os.Getenv("LOCALAPPDATA") != "":
		// Windows
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "ip-to-cloudprovider")
	case os.Getenv("XDG_DATA_HOME") != "":
		// XDG-compliant Linux
		return filepath.Join(os.Getenv("XDG_DATA_HOME"), "ip-to-cloudprovider")
	default:
		// Default: ~/.local/share/ip-to-cloudprovider
		return filepath.Join(home, ".local", "share", "ip-to-cloudprovider")
	}
}
