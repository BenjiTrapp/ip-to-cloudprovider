// Package provider defines the cloud provider registry and common utilities
// for fetching, parsing, storing, and querying IP range data.
package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

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
	Name     string
	URL      string
	Parse    ParseFunc
	Update   UpdateFunc // if set, used instead of URL+Parse
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

// FetchAndParse downloads data from the provider's URL and parses it.
func FetchAndParse(p *Provider) (*IPRange, error) {
	if p.Parse == nil {
		return nil, fmt.Errorf("provider %s has no parser", p.Name)
	}

	resp, err := http.Get(p.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", p.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", p.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response for %s: %w", p.Name, err)
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

// Save writes an IPRange to disk as JSON.
func Save(providerName string, ipRange *IPRange, dataDir string) error {
	dir := joinPath(dataDir, providerName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	data, err := json.Marshal(ipRange)
	if err != nil {
		return fmt.Errorf("marshalling %s data: %w", providerName, err)
	}

	path := joinPath(dir, "ipranges.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// Load reads an IPRange from disk.
func Load(providerName, dataDir string) (*IPRange, error) {
	path := joinPath(joinPath(dataDir, providerName), "ipranges.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var ipRange IPRange
	if err := json.Unmarshal(data, &ipRange); err != nil {
		return nil, fmt.Errorf("unmarshalling %s: %w", path, err)
	}

	return &ipRange, nil
}

// CheckIP checks if an IP is in the provider's ranges. Returns the provider
// name if found, or an empty string if not.
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

// joinPath joins path segments with the OS path separator.
func joinPath(parts ...string) string {
	return strings.Join(parts, string(os.PathSeparator))
}
