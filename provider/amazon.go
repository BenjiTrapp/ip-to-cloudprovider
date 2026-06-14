package provider

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register(Provider{
		Name:  "amazon",
		URL:   "https://ip-ranges.amazonaws.com/ip-ranges.json",
		Parse: parseAmazon,
	})
}

func parseAmazon(data []byte) (*IPRange, error) {
	var result struct {
		Prefixes []struct {
			IPPrefix string `json:"ip_prefix"`
		} `json:"prefixes"`
		IPv6Prefixes []struct {
			IPv6Prefix string `json:"ipv6_prefix"`
		} `json:"ipv6_prefixes"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing Amazon data: %w", err)
	}

	ipRange := &IPRange{}
	for _, p := range result.Prefixes {
		ipRange.IPv4 = append(ipRange.IPv4, p.IPPrefix)
	}
	for _, p := range result.IPv6Prefixes {
		ipRange.IPv6 = append(ipRange.IPv6, p.IPv6Prefix)
	}

	return ipRange, nil
}
