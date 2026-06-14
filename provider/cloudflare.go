package provider

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register(Provider{
		Name:  "cloudflare",
		URL:   "https://api.cloudflare.com/client/v4/ips",
		Parse: parseCloudflare,
	})
}

func parseCloudflare(data []byte) (*IPRange, error) {
	var result struct {
		Result struct {
			IPv4CIDRs []string `json:"ipv4_cidrs"`
			IPv6CIDRs []string `json:"ipv6_cidrs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing Cloudflare data: %w", err)
	}

	return &IPRange{
		IPv4: result.Result.IPv4CIDRs,
		IPv6: result.Result.IPv6CIDRs,
	}, nil
}
