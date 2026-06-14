package provider

import (
	"strings"
)

func init() {
	Register(Provider{
		Name:  "digitalocean",
		URL:   "https://www.digitalocean.com/geo/google.csv",
		Parse: parseDigitalOcean,
	})
}

// parseDigitalOcean parses DigitalOcean's CSV geo file.
// Format: CIDR,CountryCode,RegionCode,City,PostalCode (no header row).
func parseDigitalOcean(data []byte) (*IPRange, error) {
	lines := strings.Split(string(data), "\n")

	ipRange := &IPRange{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// CIDR is always the first comma-separated field
		cidr, _, _ := strings.Cut(line, ",")
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		if ClassifyCIDR(cidr) {
			ipRange.IPv6 = append(ipRange.IPv6, cidr)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, cidr)
		}
	}

	return ipRange, nil
}
