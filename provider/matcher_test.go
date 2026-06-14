package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMatcher(t *testing.T) {
	dir := t.TempDir()

	// Set up test data
	require.NoError(t, Save("amazon", &IPRange{
		IPv4: []string{"13.224.0.0/14", "52.94.76.0/22"},
		IPv6: []string{"2600:1f00::/24"},
	}, dir))
	require.NoError(t, Save("cloudflare", &IPRange{
		IPv4: []string{"198.41.128.0/17"},
		IPv6: []string{"2400:cb00::/32"},
	}, dir))

	m := NewMatcher(dir)
	require.NotNil(t, m)
	assert.GreaterOrEqual(t, len(m.entries), 2)
}

func TestMatcher_Match(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, Save("amazon", &IPRange{
		IPv4: []string{"13.224.0.0/14"},
		IPv6: []string{"2600:1f00::/24"},
	}, dir))
	require.NoError(t, Save("cloudflare", &IPRange{
		IPv4: []string{"198.41.128.0/17"},
		IPv6: []string{"2400:cb00::/32"},
	}, dir))

	m := NewMatcher(dir)

	tests := []struct {
		ip       string
		expected string
	}{
		{"13.224.1.1", "amazon"},
		{"198.41.200.1", "cloudflare"},
		{"2600:1f00::1", "amazon"},
		{"2400:cb00::1", "cloudflare"},
		{"1.2.3.4", ""},
		{"invalid", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.ip, func(t *testing.T) {
			assert.Equal(t, tc.expected, m.Match(tc.ip))
		})
	}
}

func TestMatcher_MatchAll(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, Save("amazon", &IPRange{
		IPv4: []string{"13.224.0.0/14"},
	}, dir))
	require.NoError(t, Save("cloudflare", &IPRange{
		IPv4: []string{"198.41.128.0/17"},
	}, dir))

	m := NewMatcher(dir)

	ips := []string{"13.224.1.1", "198.41.200.1", "1.2.3.4"}
	results := m.MatchAll(ips)

	require.Len(t, results, 3)

	// Results should be in order
	assert.Equal(t, "13.224.1.1", results[0].IP)
	assert.Equal(t, "amazon", results[0].Provider)
	assert.True(t, results[0].Match)

	assert.Equal(t, "198.41.200.1", results[1].IP)
	assert.Equal(t, "cloudflare", results[1].Provider)
	assert.True(t, results[1].Match)

	assert.Equal(t, "1.2.3.4", results[2].IP)
	assert.Equal(t, "", results[2].Provider)
	assert.False(t, results[2].Match)
}

func TestMatcher_MatchAll_Empty(t *testing.T) {
	dir := t.TempDir()
	m := NewMatcher(dir)

	results := m.MatchAll([]string{})
	assert.Empty(t, results)
}

func TestMatcher_SkipsBrokenProviders(t *testing.T) {
	dir := t.TempDir()

	// Only save one provider, the rest will fail to load
	require.NoError(t, Save("amazon", &IPRange{
		IPv4: []string{"13.224.0.0/14"},
	}, dir))

	m := NewMatcher(dir)
	assert.Equal(t, "amazon", m.Match("13.224.1.1"))
	assert.Equal(t, "", m.Match("198.41.200.1")) // cloudflare not loaded
}

func BenchmarkMatcher_Match(b *testing.B) {
	dir := b.TempDir()

	// Simulate realistic data volume
	Save("amazon", &IPRange{IPv4: generateCIDRs(500)}, dir)
	Save("cloudflare", &IPRange{IPv4: generateCIDRs(100)}, dir)
	Save("google", &IPRange{IPv4: generateCIDRs(300)}, dir)
	Save("microsoft", &IPRange{IPv4: generateCIDRs(1000)}, dir)

	m := NewMatcher(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match("200.100.50.1")
	}
}

func BenchmarkMatcher_MatchAll(b *testing.B) {
	dir := b.TempDir()

	Save("amazon", &IPRange{IPv4: generateCIDRs(500)}, dir)
	Save("cloudflare", &IPRange{IPv4: generateCIDRs(100)}, dir)
	Save("google", &IPRange{IPv4: generateCIDRs(300)}, dir)

	m := NewMatcher(dir)

	ips := make([]string, 100)
	for i := range ips {
		ips[i] = fmt.Sprintf("%d.%d.%d.1", (i*7)%256, (i*13)%256, (i*17)%256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MatchAll(ips)
	}
}

func generateCIDRs(n int) []string {
	cidrs := make([]string, n)
	for i := range cidrs {
		cidrs[i] = fmt.Sprintf("%d.%d.0.0/16", (i*3)%256, (i*7)%256)
	}
	return cidrs
}
