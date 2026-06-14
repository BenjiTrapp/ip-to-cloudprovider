package provider

import "strings"

func init() {
	Register(Provider{
		Name:  "openai",
		URL:   "https://openai.com/gptbot-ranges.txt",
		Parse: parseOpenAI,
	})
}

// parseOpenAI parses OpenAI's plain-text CIDR list (one per line).
func parseOpenAI(data []byte) (*IPRange, error) {
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
