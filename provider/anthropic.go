package provider

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

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
