package provider

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// privateAndReservedNetworks contains CIDRs that should never appear in
// provider data (private, loopback, documentation, link-local, etc.).
var privateAndReservedNetworks []*net.IPNet

func init() {
	reserved := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"192.0.2.0/24",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"fc00::/7",
		"fe80::/10",
		"2001:db8::/32",
		"::1/128",
	}
	for _, cidr := range reserved {
		_, network, _ := net.ParseCIDR(cidr)
		privateAndReservedNetworks = append(privateAndReservedNetworks, network)
	}
}

// isPrivateOrReserved returns true if the given CIDR falls within a
// private or reserved address space.
func isPrivateOrReserved(cidr string) bool {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	for _, reserved := range privateAndReservedNetworks {
		if reserved.Contains(ip) || reserved.Contains(network.IP) {
			return true
		}
	}
	return false
}

const anthropicDocsURL = "https://docs.anthropic.com/en/api/ip-addresses"

// cidrRegex matches IPv4 and IPv6 CIDR notation in text.
var cidrRegex = regexp.MustCompile(`(?:(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}|[0-9a-fA-F:]+::/\d{1,3})`)

func init() {
	Register(Provider{
		Name:   "anthropic",
		URL:    anthropicDocsURL,
		Update: updateAnthropic,
	})
}

// updateAnthropic fetches the Anthropic docs page and extracts CIDR ranges.
// Anthropic does not provide a machine-readable API; their IP ranges are
// documented at https://docs.anthropic.com/en/api/ip-addresses
func updateAnthropic(dataDir string) error {
	body, err := Fetch(anthropicDocsURL)
	if err != nil {
		return fmt.Errorf("fetching Anthropic IP docs: %w", err)
	}

	ipRange, err := parseAnthropic(body)
	if err != nil {
		return err
	}

	return Save("anthropic", ipRange, dataDir)
}

// parseAnthropic extracts CIDR ranges from the Anthropic docs page content.
// It uses regex to find valid CIDR patterns and deduplicates them.
func parseAnthropic(data []byte) (*IPRange, error) {
	content := string(data)

	// Remove phased-out IP section to avoid including deprecated ranges
	if idx := strings.Index(content, "Phased out"); idx != -1 {
		content = content[:idx]
	}
	if idx := strings.Index(content, "phased out"); idx != -1 {
		content = content[:idx]
	}

	matches := cidrRegex.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no CIDR ranges found in Anthropic docs page")
	}

	ipRange := &IPRange{}
	seen := make(map[string]bool)

	for _, cidr := range matches {
		cidr = strings.TrimSpace(cidr)
		// Validate it's actually a valid CIDR
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if seen[cidr] {
			continue
		}
		// Skip private/reserved ranges (e.g. documentation examples)
		if isPrivateOrReserved(cidr) {
			continue
		}
		seen[cidr] = true

		if ClassifyCIDR(cidr) {
			ipRange.IPv6 = append(ipRange.IPv6, cidr)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, cidr)
		}
	}

	if len(ipRange.IPv4) == 0 && len(ipRange.IPv6) == 0 {
		return nil, fmt.Errorf("no valid CIDR ranges extracted from Anthropic docs")
	}

	return ipRange, nil
}
