package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// OpenAI update (multi-file merge) end-to-end
// ---------------------------------------------------------------------------

func TestUpdateOpenAI(t *testing.T) {
	gptbot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"prefixes":[{"ipv4Prefix":"23.98.142.176/28"},{"ipv6Prefix":"2607:f8b0:4000::/36"}]}`)
	}))
	defer gptbot.Close()
	searchbot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"prefixes":[{"ipv4Prefix":"40.84.180.224/28"}]}`)
	}))
	defer searchbot.Close()

	orig := openaiBotURLs
	openaiBotURLs = []string{gptbot.URL, searchbot.URL}
	defer func() { openaiBotURLs = orig }()

	dir := t.TempDir()
	require.NoError(t, updateOpenAI(dir))

	loaded, err := Load("openai", dir)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"23.98.142.176/28", "40.84.180.224/28"}, loaded.IPv4)
	assert.Equal(t, []string{"2607:f8b0:4000::/36"}, loaded.IPv6)
}

func TestUpdateOpenAIAllFail(t *testing.T) {
	orig := openaiBotURLs
	openaiBotURLs = []string{"http://127.0.0.1:1"}
	defer func() { openaiBotURLs = orig }()

	assert.Error(t, updateOpenAI(t.TempDir()))
}

// ---------------------------------------------------------------------------
// Microsoft update (multi-cloud merge, required vs optional) via stub
// ---------------------------------------------------------------------------

func TestUpdateMicrosoft(t *testing.T) {
	orig := fetchMicrosoftCloud
	defer func() { fetchMicrosoftCloud = orig }()

	t.Run("merges and deduplicates across clouds", func(t *testing.T) {
		fetchMicrosoftCloud = func(id string) (*IPRange, error) {
			switch id {
			case "56519": // Public
				return &IPRange{IPv4: []string{"10.0.0.0/8", "192.0.2.0/24"}}, nil
			case "57063": // USGov (overlaps Public on 10.0.0.0/8)
				return &IPRange{IPv4: []string{"10.0.0.0/8"}, IPv6: []string{"2001:db8::/32"}}, nil
			default: // China / Germany
				return &IPRange{IPv4: []string{"203.0.113.0/24"}}, nil
			}
		}

		dir := t.TempDir()
		require.NoError(t, updateMicrosoft(dir))

		loaded, err := Load("microsoft", dir)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"10.0.0.0/8", "192.0.2.0/24", "203.0.113.0/24"}, loaded.IPv4)
		assert.Equal(t, []string{"2001:db8::/32"}, loaded.IPv6)
	})

	t.Run("required cloud failure aborts the update", func(t *testing.T) {
		fetchMicrosoftCloud = func(id string) (*IPRange, error) {
			if id == "56519" { // Public is required
				return nil, fmt.Errorf("boom")
			}
			return &IPRange{IPv4: []string{"10.0.0.0/8"}}, nil
		}
		err := updateMicrosoft(t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Public")
	})

	t.Run("optional cloud failure is skipped", func(t *testing.T) {
		fetchMicrosoftCloud = func(id string) (*IPRange, error) {
			switch id {
			case "56519", "57063": // required clouds succeed
				return &IPRange{IPv4: []string{"10.0.0.0/8"}}, nil
			default: // optional clouds fail
				return nil, fmt.Errorf("blocked")
			}
		}
		dir := t.TempDir()
		require.NoError(t, updateMicrosoft(dir))
		loaded, err := Load("microsoft", dir)
		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8"}, loaded.IPv4)
	})
}

func TestDiscoverMicrosoftDownloadURLWrapper(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
			<a href="https://download.microsoft.com/download/x/ServiceTags_Public_20240101.json">Download</a>
		</body></html>`)
	}))
	defer page.Close()

	orig := microsoftPageURL
	microsoftPageURL = func(id string) string { return page.URL }
	defer func() { microsoftPageURL = orig }()

	url, err := discoverMicrosoftDownloadURL("56519")
	require.NoError(t, err)
	assert.Contains(t, url, "ServiceTags_Public")
}

// ---------------------------------------------------------------------------
// CheckProvider / CheckAll (health check)
// ---------------------------------------------------------------------------

func TestCheckProvider(t *testing.T) {
	t.Run("parse-style provider succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "10.0.0.0/8\n2001:db8::/32\n")
		}))
		defer server.Close()

		p := &Provider{Name: "parsecheck", URL: server.URL, Parse: ParsePlainTextCIDRs}
		ranges, err := CheckProvider(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8"}, ranges.IPv4)
		assert.Equal(t, []string{"2001:db8::/32"}, ranges.IPv6)
	})

	t.Run("parse-style provider surfaces HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		p := &Provider{Name: "parsecheck", URL: server.URL, Parse: ParsePlainTextCIDRs}
		_, err := CheckProvider(p)
		assert.Error(t, err)
	})

	t.Run("update-style provider succeeds without persisting", func(t *testing.T) {
		bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"prefixes":[{"ipv4Prefix":"23.98.142.176/28"}]}`)
		}))
		defer bot.Close()

		orig := openaiBotURLs
		openaiBotURLs = []string{bot.URL}
		defer func() { openaiBotURLs = orig }()

		p := ByName("openai")
		require.NotNil(t, p)
		ranges, err := CheckProvider(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"23.98.142.176/28"}, ranges.IPv4)
	})

	t.Run("update-style provider surfaces failure", func(t *testing.T) {
		orig := openaiBotURLs
		openaiBotURLs = []string{"http://127.0.0.1:1"}
		defer func() { openaiBotURLs = orig }()

		p := ByName("openai")
		require.NotNil(t, p)
		_, err := CheckProvider(p)
		assert.Error(t, err)
	})
}

func TestCheckOne(t *testing.T) {
	t.Run("healthy provider", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "10.0.0.0/8\n")
		}))
		defer server.Close()

		res := checkOne(&Provider{Name: "ok", URL: server.URL, Parse: ParsePlainTextCIDRs})
		assert.True(t, res.OK)
		assert.Equal(t, 1, res.IPv4)
		assert.Empty(t, res.Err)
	})

	t.Run("empty result is unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "\n\n")
		}))
		defer server.Close()

		res := checkOne(&Provider{Name: "empty", URL: server.URL, Parse: ParsePlainTextCIDRs})
		assert.False(t, res.OK)
		assert.Equal(t, "no IP ranges returned", res.Err)
	})

	t.Run("fetch error is unhealthy", func(t *testing.T) {
		res := checkOne(&Provider{Name: "down", URL: "http://127.0.0.1:1", Parse: ParsePlainTextCIDRs})
		assert.False(t, res.OK)
		assert.NotEmpty(t, res.Err)
	})
}
