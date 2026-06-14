package provider

import (
	"net"
	"sync"
)

// concurrencyThreshold is the minimum number of IPs before spawning goroutines.
const concurrencyThreshold = 50

// Matcher is a pre-loaded, optimized IP range matcher that parses all CIDRs
// upfront for fast repeated lookups.
type Matcher struct {
	entries []matcherEntry
	loaded  int // number of providers successfully loaded
}

type matcherEntry struct {
	name string
	nets []*net.IPNet
}

// NewMatcher loads all provider IP ranges from disk and pre-parses CIDR
// networks for fast lookup. Providers that fail to load are silently skipped.
func NewMatcher(dataDir string) *Matcher {
	m := &Matcher{}
	for _, p := range Registry {
		ipRange, err := Load(p.Name, dataDir)
		if err != nil {
			continue
		}

		var nets []*net.IPNet
		for _, cidr := range ipRange.IPv4 {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				nets = append(nets, ipNet)
			}
		}
		for _, cidr := range ipRange.IPv6 {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				nets = append(nets, ipNet)
			}
		}

		if len(nets) > 0 {
			m.entries = append(m.entries, matcherEntry{name: p.Name, nets: nets})
			m.loaded++
		}
	}
	return m
}

// Loaded returns the number of providers successfully loaded.
func (m *Matcher) Loaded() int {
	return m.loaded
}

// Match returns the provider name for the given IP, or empty string if not found.
func (m *Matcher) Match(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}
	for _, entry := range m.entries {
		for _, ipNet := range entry.nets {
			if ipNet.Contains(parsedIP) {
				return entry.name
			}
		}
	}
	return ""
}

// MatchResult holds the result of an IP lookup.
type MatchResult struct {
	IP       string `json:"ip"`
	Provider string `json:"provider,omitempty"`
	Match    bool   `json:"match"`
}

// MatchAll checks multiple IPs and returns results in order.
// Uses concurrency only when the batch is large enough to benefit.
func (m *Matcher) MatchAll(ips []string) []MatchResult {
	results := make([]MatchResult, len(ips))

	if len(ips) < concurrencyThreshold {
		// Sequential for small batches (avoids goroutine overhead)
		for i, ip := range ips {
			name := m.Match(ip)
			results[i] = MatchResult{IP: ip, Provider: name, Match: name != ""}
		}
		return results
	}

	// Concurrent for large batches
	var wg sync.WaitGroup
	for i, ip := range ips {
		wg.Add(1)
		go func(idx int, addr string) {
			defer wg.Done()
			name := m.Match(addr)
			results[idx] = MatchResult{IP: addr, Provider: name, Match: name != ""}
		}(i, ip)
	}
	wg.Wait()

	return results
}

// Summary returns aggregate match statistics.
func Summary(results []MatchResult) map[string]int {
	counts := make(map[string]int)
	for _, r := range results {
		if r.Match {
			counts[r.Provider]++
		} else {
			counts["unknown"]++
		}
	}
	return counts
}
