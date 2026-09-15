package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CheckResult is the outcome of a single provider health check.
type CheckResult struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	IPv4     int    `json:"ipv4"`
	IPv6     int    `json:"ipv6"`
	Err      string `json:"error,omitempty"`
}

// CheckProvider live-fetches and parses a provider's upstream source WITHOUT
// persisting data, returning the parsed ranges. Update-style providers (which
// normally write directly to disk) are exercised against a throwaway temp
// directory so their full multi-step fetch path is validated. This is the basis
// for the healthcheck command, which detects upstream endpoint or format
// changes early — a retired URL, an HTTP 403/404, or a changed payload format
// all surface here as an error or an empty result.
func CheckProvider(p *Provider) (*IPRange, error) {
	if p.Update == nil {
		return FetchAndParse(p)
	}

	tmp, err := os.MkdirTemp("", "ip2cp-check-")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := p.Update(tmp); err != nil {
		return nil, err
	}

	// Read the freshly written file directly (not via Load) so the embedded
	// snapshot can never mask a genuinely failed or empty update.
	data, err := os.ReadFile(filepath.Join(tmp, p.Name, "ipranges.json"))
	if err != nil {
		return nil, fmt.Errorf("reading updated data: %w", err)
	}
	var ipRange IPRange
	if err := json.Unmarshal(data, &ipRange); err != nil {
		return nil, fmt.Errorf("parsing updated data: %w", err)
	}
	return &ipRange, nil
}

// CheckAll runs a live health check against every registered provider and
// returns the results in registry order. A provider is considered unhealthy if
// the fetch/parse fails or yields zero usable ranges.
func CheckAll() []CheckResult {
	results := make([]CheckResult, 0, len(Registry))
	for i := range Registry {
		results = append(results, checkOne(&Registry[i]))
	}
	return results
}

// checkOne health-checks a single provider and classifies the outcome.
func checkOne(p *Provider) CheckResult {
	res := CheckResult{Provider: p.Name}
	ranges, err := CheckProvider(p)
	switch {
	case err != nil:
		res.Err = err.Error()
	case ranges == nil || (len(ranges.IPv4) == 0 && len(ranges.IPv6) == 0):
		res.Err = "no IP ranges returned"
	default:
		res.OK = true
		res.IPv4 = len(ranges.IPv4)
		res.IPv6 = len(ranges.IPv6)
	}
	return res
}
