package provider

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register(Provider{
		Name:  "google",
		URL:   "https://www.gstatic.com/ipranges/goog.txt",
		Parse: parseGoogleTxt,
	})
	Register(Provider{
		Name:  "googlecloud",
		URL:   "https://www.gstatic.com/ipranges/cloud.json",
		Parse: parseGoogleJSON,
	})
	Register(Provider{
		Name:  "googlebot",
		URL:   "https://developers.google.com/search/apis/ipranges/googlebot.json",
		Parse: parseGoogleJSON,
	})
}

// parseGoogleTxt parses Google's plain-text IP range list (one CIDR per line).
func parseGoogleTxt(data []byte) (*IPRange, error) {
	return ParsePlainTextCIDRs(data)
}

// parseGoogleJSON parses Google's JSON IP range format (cloud.json, googlebot.json).
func parseGoogleJSON(data []byte) (*IPRange, error) {
	var result struct {
		Prefixes []struct {
			IPv4Prefix string `json:"ipv4Prefix"`
			IPv6Prefix string `json:"ipv6Prefix"`
		} `json:"prefixes"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing Google JSON data: %w", err)
	}

	ipRange := &IPRange{}
	for _, prefix := range result.Prefixes {
		if prefix.IPv4Prefix != "" {
			ipRange.IPv4 = append(ipRange.IPv4, prefix.IPv4Prefix)
		}
		if prefix.IPv6Prefix != "" {
			ipRange.IPv6 = append(ipRange.IPv6, prefix.IPv6Prefix)
		}
	}

	return ipRange, nil
}
