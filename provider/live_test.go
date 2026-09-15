//go:build live

// Package provider live smoke tests. These hit the real upstream endpoints and
// are excluded from the normal test run (they require network and can be slow
// or rate-limited). Run them explicitly to detect when a provider changes its
// endpoint or payload format:
//
//	go test -tags live ./provider/ -run TestLive -v
//
// The same check is available at runtime via `ip-to-cloudprovider healthcheck`.
package provider

import "testing"

// TestLiveAllProviders verifies every registered provider's upstream source is
// reachable and still parses into a non-empty set of IP ranges. A retired URL,
// an HTTP 403/404, or a changed payload format will fail this test.
func TestLiveAllProviders(t *testing.T) {
	results := CheckAll()
	if len(results) == 0 {
		t.Fatal("no providers registered")
	}

	for _, r := range results {
		r := r
		t.Run(r.Provider, func(t *testing.T) {
			if !r.OK {
				t.Fatalf("provider %q unhealthy: %s", r.Provider, r.Err)
			}
			t.Logf("%s: IPv4=%d IPv6=%d", r.Provider, r.IPv4, r.IPv6)
		})
	}
}
