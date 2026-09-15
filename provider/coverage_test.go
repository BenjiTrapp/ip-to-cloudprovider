package provider

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Data-availability helpers
// ---------------------------------------------------------------------------

func TestHasDataAndHasAnyData(t *testing.T) {
	dir := t.TempDir()

	// Ensure no embedded fallback interferes with this test.
	origEmbedded := EmbeddedData
	EmbeddedData = nil
	defer func() { EmbeddedData = origEmbedded }()

	assert.False(t, HasData("amazon", dir))
	assert.False(t, HasAnyData(dir))

	require.NoError(t, Save("amazon", &IPRange{IPv4: []string{"10.0.0.0/8"}}, dir))

	assert.True(t, HasData("amazon", dir))
	assert.True(t, HasAnyData(dir))
	assert.False(t, HasData("cloudflare", dir))
}

func TestEmbeddedFallback(t *testing.T) {
	origEmbedded := EmbeddedData
	defer func() { EmbeddedData = origEmbedded }()

	EmbeddedData = fstest.MapFS{
		"amazon/ipranges.json": &fstest.MapFile{Data: []byte(`{"ipv4":["1.2.3.0/24"],"ipv6":null}`)},
	}

	assert.True(t, hasEmbedded("amazon"))
	assert.False(t, hasEmbedded("nonexistent"))

	// HasData falls back to the embedded snapshot when no file exists on disk.
	assert.True(t, HasData("amazon", t.TempDir()))

	ranges, err := embeddedRange("amazon")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.2.3.0/24"}, ranges.IPv4)

	_, err = embeddedRange("nonexistent")
	assert.Error(t, err)
}

func TestDefaultDataDir(t *testing.T) {
	t.Setenv("IP2CP_DATA_DIR", "/tmp/custom-ip2cp")
	assert.Equal(t, "/tmp/custom-ip2cp", DefaultDataDir())

	t.Setenv("IP2CP_DATA_DIR", "")
	assert.NotEmpty(t, DefaultDataDir())
}

// ---------------------------------------------------------------------------
// GitHub helper
// ---------------------------------------------------------------------------

func TestIsGitHubProvider(t *testing.T) {
	for _, name := range []string{"github", "githubactions", "githubhooks", "githubpages"} {
		assert.True(t, IsGitHubProvider(name), name)
	}
	for _, name := range []string{"amazon", "google", "", "gitlab"} {
		assert.False(t, IsGitHubProvider(name), name)
	}
}

// ---------------------------------------------------------------------------
// Matcher: Loaded, Summary, and concurrent MatchAll
// ---------------------------------------------------------------------------

func TestMatcherLoadedAndSummary(t *testing.T) {
	dir := t.TempDir()
	origEmbedded := EmbeddedData
	EmbeddedData = nil
	defer func() { EmbeddedData = origEmbedded }()

	require.NoError(t, Save("amazon", &IPRange{IPv4: []string{"13.0.0.0/8"}}, dir))
	require.NoError(t, Save("google", &IPRange{IPv4: []string{"8.8.8.0/24"}}, dir))

	m := NewMatcher(dir)
	assert.Equal(t, 2, m.Loaded())

	results := m.MatchAll([]string{"13.1.1.1", "8.8.8.8", "192.168.1.1"})
	summary := Summary(results)
	assert.Equal(t, 1, summary["amazon"])
	assert.Equal(t, 1, summary["google"])
	assert.Equal(t, 1, summary["unknown"])
}

func TestMatchAllConcurrent(t *testing.T) {
	dir := t.TempDir()
	origEmbedded := EmbeddedData
	EmbeddedData = nil
	defer func() { EmbeddedData = origEmbedded }()

	require.NoError(t, Save("amazon", &IPRange{IPv4: []string{"13.0.0.0/8"}}, dir))
	m := NewMatcher(dir)

	// Exceed concurrencyThreshold (50) to hit the goroutine path, and assert
	// results stay aligned with their input index.
	const n = 120
	ips := make([]string, n)
	for i := range ips {
		if i%2 == 0 {
			ips[i] = "13.0.0.1"
		} else {
			ips[i] = fmt.Sprintf("192.168.0.%d", i%256)
		}
	}

	results := m.MatchAll(ips)
	require.Len(t, results, n)
	for i, r := range results {
		assert.Equal(t, ips[i], r.IP)
		if i%2 == 0 {
			assert.True(t, r.Match)
			assert.Equal(t, "amazon", r.Provider)
		} else {
			assert.False(t, r.Match)
		}
	}
}
