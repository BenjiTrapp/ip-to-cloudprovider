package provider

import (
	"fmt"
	"strings"
)

const (
	alibabaIPv4URL = "https://raw.githubusercontent.com/ipverse/asn-ip/master/as/45102/ipv4-aggregated.txt"
	alibabaIPv6URL = "https://raw.githubusercontent.com/ipverse/asn-ip/master/as/45102/ipv6-aggregated.txt"
)

func init() {
	Register(Provider{
		Name:   "alibaba",
		URL:    alibabaIPv4URL,
		Update: updateAlibaba,
	})
}

// updateAlibaba fetches both IPv4 and IPv6 aggregated CIDR lists for
// Alibaba Cloud (AS45102) and merges them.
func updateAlibaba(dataDir string) error {
	ipv4Data, err := Fetch(alibabaIPv4URL)
	if err != nil {
		return fmt.Errorf("fetching Alibaba IPv4 ranges: %w", err)
	}

	ipv6Data, err := Fetch(alibabaIPv6URL)
	if err != nil {
		return fmt.Errorf("fetching Alibaba IPv6 ranges: %w", err)
	}

	ipRange := &IPRange{
		IPv4: parseCommentedCIDRs(string(ipv4Data)),
		IPv6: parseCommentedCIDRs(string(ipv6Data)),
	}

	return Save("alibaba", ipRange, dataDir)
}

// parseAlibaba parses the plain-text format with # comment lines.
// Used for testing and direct parsing when data is already fetched.
func parseAlibaba(data []byte) (*IPRange, error) {
	cidrs := parseCommentedCIDRs(string(data))
	ipRange := &IPRange{}
	for _, cidr := range cidrs {
		if ClassifyCIDR(cidr) {
			ipRange.IPv6 = append(ipRange.IPv6, cidr)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, cidr)
		}
	}
	return ipRange, nil
}

// parseCommentedCIDRs parses lines of CIDRs, skipping comment lines (# prefix)
// and empty lines.
func parseCommentedCIDRs(text string) []string {
	lines := strings.Split(text, "\n")
	var cidrs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cidrs = append(cidrs, line)
	}
	return cidrs
}
