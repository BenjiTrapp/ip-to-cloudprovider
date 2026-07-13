package reputation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregate(t *testing.T) {
	tests := []struct {
		name        string
		results     []SourceResult
		wantVerdict Verdict
		wantScore   int
	}{
		{
			name:        "no sources answered",
			results:     []SourceResult{{Source: "a", Err: "boom"}},
			wantVerdict: VerdictUnknown,
			wantScore:   0,
		},
		{
			name:        "all clean",
			results:     []SourceResult{{Source: "a"}, {Source: "b"}},
			wantVerdict: VerdictClean,
			wantScore:   0,
		},
		{
			name:        "listed low score is suspicious",
			results:     []SourceResult{{Source: "a", Listed: true, Score: 10}},
			wantVerdict: VerdictSuspicious,
			wantScore:   10,
		},
		{
			name:        "high score is malicious",
			results:     []SourceResult{{Source: "a", Listed: true, Score: 90}},
			wantVerdict: VerdictMalicious,
			wantScore:   90,
		},
		{
			name:        "errored source ignored, other clean",
			results:     []SourceResult{{Source: "a", Err: "timeout"}, {Source: "b"}},
			wantVerdict: VerdictClean,
			wantScore:   0,
		},
		{
			name:        "max score wins",
			results:     []SourceResult{{Source: "a", Listed: true, Score: 30}, {Source: "b", Listed: true, Score: 80}},
			wantVerdict: VerdictMalicious,
			wantScore:   80,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := aggregate("1.2.3.4", tc.results)
			assert.Equal(t, tc.wantVerdict, got.Verdict)
			assert.Equal(t, tc.wantScore, got.Score)
			assert.Equal(t, "1.2.3.4", got.IP)
		})
	}
}

func TestReverseIPv4(t *testing.T) {
	tests := []struct {
		ip     string
		want   string
		wantOK bool
	}{
		{"1.2.3.4", "4.3.2.1", true},
		{"8.8.8.8", "8.8.8.8", true},
		{"127.0.0.1", "1.0.0.127", true},
		{"not-an-ip", "", false},
		{"2001:db8::1", "", false}, // IPv6 not supported
	}
	for _, tc := range tests {
		got, ok := reverseIPv4(tc.ip)
		assert.Equal(t, tc.wantOK, ok, tc.ip)
		assert.Equal(t, tc.want, got, tc.ip)
	}
}

func TestDNSBLResponseCodes(t *testing.T) {
	tests := []struct {
		addr        string
		wantListing bool
		wantError   bool
	}{
		{"127.0.0.2", true, false},       // typical listing
		{"127.0.0.10", true, false},      // another sub-list
		{"127.255.255.254", false, true}, // Spamhaus public-resolver error
		{"127.255.255.252", false, true}, // rate-limit error
		{"93.184.216.34", false, false},  // arbitrary non-DNSBL address
	}
	for _, tc := range tests {
		assert.Equal(t, tc.wantListing, isDNSBLListing(tc.addr), "listing %s", tc.addr)
		assert.Equal(t, tc.wantError, isDNSBLError(tc.addr), "error %s", tc.addr)
	}
}

func TestDNSBLCheck_NonIPv4(t *testing.T) {
	d := NewDNSBL("test", "example.invalid", 50)
	res := d.Check(context.Background(), "2001:db8::1")
	assert.False(t, res.Listed)
	assert.Empty(t, res.Err) // IPv6 is a no-op, not an error
}

func TestDefaultConfigSources(t *testing.T) {
	// Ensure no env key leaks into this test.
	t.Setenv(abuseIPDBKeyEnv, "")

	cfg := DefaultConfig()
	checker := NewChecker(cfg)
	assert.True(t, checker.Enabled())
	// All defaults are keyless DNSBLs; AbuseIPDB is off without a key.
	assert.Equal(t, len(defaultDNSBLs), len(checker.Sources()))
	assert.Contains(t, checker.Sources(), "spamhaus-zen")
}

func TestConfigDisableDNSBL(t *testing.T) {
	off := false
	cfg := Config{
		DNSBLs: []DNSBLConfig{
			{Name: "keep", Zone: "keep.example", Score: 50},
			{Name: "drop", Zone: "drop.example", Score: 50, Enabled: &off},
		},
	}
	names := NewChecker(cfg).Sources()
	assert.Equal(t, []string{"keep"}, names)
}

func TestConfigAbuseIPDBRequiresKey(t *testing.T) {
	t.Setenv(abuseIPDBKeyEnv, "")

	// Enabled but no key -> not active.
	cfg := Config{AbuseIPDB: AbuseIPDBConfig{Enabled: true}}
	cfg.applyDefaults()
	assert.NotContains(t, NewChecker(cfg).Sources(), "abuseipdb")

	// Enabled with key -> active.
	cfg = Config{AbuseIPDB: AbuseIPDBConfig{Enabled: true, APIKey: "secret"}}
	cfg.applyDefaults()
	assert.Contains(t, NewChecker(cfg).Sources(), "abuseipdb")
}

func TestLoadConfig_MissingFileUsesDefaults(t *testing.T) {
	t.Setenv(abuseIPDBKeyEnv, "")
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.DNSBLs)
}

func TestLoadConfig_FromFile(t *testing.T) {
	t.Setenv(abuseIPDBKeyEnv, "")
	path := filepath.Join(t.TempDir(), "reputation.yaml")
	content := `
dnsbls:
  - name: only-one
    zone: one.example
    score: 42
abuseipdb:
  enabled: true
  api_key: from-file
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	names := NewChecker(cfg).Sources()
	assert.Contains(t, names, "only-one")
	assert.Contains(t, names, "abuseipdb")
	assert.Len(t, cfg.DNSBLs, 1)
}

func TestLoadConfig_EnvKeyFallback(t *testing.T) {
	t.Setenv(abuseIPDBKeyEnv, "env-key")
	path := filepath.Join(t.TempDir(), "reputation.yaml")
	content := "abuseipdb:\n  enabled: true\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "env-key", cfg.AbuseIPDB.APIKey)
	assert.Contains(t, NewChecker(cfg).Sources(), "abuseipdb")
}
