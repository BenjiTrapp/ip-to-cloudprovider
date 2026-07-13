// Package reputation checks whether an IP address is considered malicious by
// one or more threat-intelligence sources.
//
// Two kinds of sources are supported:
//
//   - DNSBL (DNS blocklists such as Spamhaus, SpamCop, UCEPROTECT). These need
//     no API key or registration and are enabled by default.
//   - API-based services (AbuseIPDB). These require an API key, supplied via
//     the config file or an environment variable, and are opt-in.
//
// Which sources are active is controlled entirely by the config file (see
// config.go). A Checker fans out across all configured sources concurrently
// and aggregates the individual results into a single verdict per IP.
package reputation

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Verdict is the aggregated reputation classification for an IP.
type Verdict string

const (
	// VerdictClean means no source flagged the IP.
	VerdictClean Verdict = "clean"
	// VerdictSuspicious means at least one source flagged the IP with a low score.
	VerdictSuspicious Verdict = "suspicious"
	// VerdictMalicious means at least one high-confidence source flagged the IP.
	VerdictMalicious Verdict = "malicious"
	// VerdictUnknown means no source could return a usable answer.
	VerdictUnknown Verdict = "unknown"
)

// Score thresholds used to derive a Verdict from the maximum source score.
const (
	maliciousThreshold  = 75
	suspiciousThreshold = 25
)

// checkTimeout bounds the total time spent checking a single IP across all sources.
const checkTimeout = 10 * time.Second

// SourceResult is the outcome of one source checking one IP.
type SourceResult struct {
	Source     string   `json:"source"`
	Listed     bool     `json:"listed"`
	Score      int      `json:"score,omitempty"`      // 0-100 confidence, where meaningful
	Categories []string `json:"categories,omitempty"` // optional labels (e.g. abuse types)
	Detail     string   `json:"detail,omitempty"`     // human-readable extra info
	Err        string   `json:"error,omitempty"`      // set if the lookup failed
}

// Report aggregates every source result for a single IP.
type Report struct {
	IP      string         `json:"ip"`
	Verdict Verdict        `json:"verdict"`
	Score   int            `json:"score"` // highest score across successful sources
	Sources []SourceResult `json:"sources"`
}

// Source checks the reputation of a single IP.
type Source interface {
	// Name returns a stable identifier for the source (e.g. "spamhaus-zen").
	Name() string
	// Check looks up the given IP. It should return a SourceResult rather than
	// an error for "not listed" outcomes; the Err field carries genuine failures.
	Check(ctx context.Context, ip string) SourceResult
}

// Checker runs a set of sources against IPs and aggregates their results.
type Checker struct {
	sources []Source
}

// NewChecker builds a Checker from a Config, wiring up every active source.
func NewChecker(cfg Config) *Checker {
	return &Checker{sources: cfg.sources()}
}

// Sources returns the names of the active sources, in order.
func (c *Checker) Sources() []string {
	names := make([]string, len(c.sources))
	for i, s := range c.sources {
		names[i] = s.Name()
	}
	return names
}

// Enabled reports whether the checker has at least one active source.
func (c *Checker) Enabled() bool {
	return len(c.sources) > 0
}

// Check runs all sources against a single IP concurrently and aggregates them.
func (c *Checker) Check(ctx context.Context, ip string) Report {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	results := make([]SourceResult, len(c.sources))
	var wg sync.WaitGroup
	for i, s := range c.sources {
		wg.Add(1)
		go func(idx int, src Source) {
			defer wg.Done()
			results[idx] = src.Check(ctx, ip)
		}(i, s)
	}
	wg.Wait()

	return aggregate(ip, results)
}

// CheckAll runs the checker against multiple IPs, returning reports in order.
func (c *Checker) CheckAll(ctx context.Context, ips []string) []Report {
	reports := make([]Report, len(ips))
	for i, ip := range ips {
		reports[i] = c.Check(ctx, ip)
	}
	return reports
}

// aggregate turns per-source results into a single Report with an overall verdict.
func aggregate(ip string, results []SourceResult) Report {
	report := Report{IP: ip, Sources: results, Verdict: VerdictUnknown}

	var (
		maxScore  int
		anyOK     bool // at least one source answered without error
		anyListed bool
	)
	for _, r := range results {
		if r.Err != "" {
			continue
		}
		anyOK = true
		if r.Listed {
			anyListed = true
		}
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	report.Score = maxScore
	switch {
	case !anyOK:
		report.Verdict = VerdictUnknown
	case maxScore >= maliciousThreshold:
		report.Verdict = VerdictMalicious
	case anyListed || maxScore >= suspiciousThreshold:
		report.Verdict = VerdictSuspicious
	default:
		report.Verdict = VerdictClean
	}

	// Stable ordering for deterministic output.
	sort.SliceStable(report.Sources, func(i, j int) bool {
		return report.Sources[i].Source < report.Sources[j].Source
	})

	return report
}
