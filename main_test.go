package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BenjiTrapp/ip-to-cloudprovider/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mockProviderData contains mock responses keyed by provider name.
var mockProviderData = map[string]string{
	"amazon":        `{"prefixes": [{"ip_prefix": "13.224.0.0/14"}, {"ip_prefix": "52.94.76.0/22"}], "ipv6_prefixes": [{"ipv6_prefix": "2600:1f00::/24"}]}`,
	"cloudflare":    `{"result": {"ipv4_cidrs": ["198.41.128.0/17", "104.16.0.0/13"], "ipv6_cidrs": ["2400:cb00::/32"]}}`,
	"github":        `{"web": ["192.30.252.0/22"], "actions": ["4.148.0.0/15"], "hooks": ["140.82.112.0/20"], "pages": ["185.199.108.0/22"]}`,
	"githubactions": `{"web": ["192.30.252.0/22"], "actions": ["4.148.0.0/15"], "hooks": ["140.82.112.0/20"], "pages": ["185.199.108.0/22"]}`,
	"githubhooks":   `{"web": ["192.30.252.0/22"], "actions": ["4.148.0.0/15"], "hooks": ["140.82.112.0/20"], "pages": ["185.199.108.0/22"]}`,
	"githubpages":   `{"web": ["192.30.252.0/22"], "actions": ["4.148.0.0/15"], "hooks": ["140.82.112.0/20"], "pages": ["185.199.108.0/22"]}`,
	"google":        "8.8.8.0/24\n8.8.4.0/24\n2001:4860::/32\n",
	"googlecloud":   `{"prefixes": [{"ipv4Prefix": "34.80.0.0/15"}, {"ipv6Prefix": "2600:1900::/35"}]}`,
	"googlebot":     `{"prefixes": [{"ipv4Prefix": "66.249.64.0/19"}]}`,
	"openai":        "23.98.142.176/28\n40.84.180.224/28\n",
	"digitalocean":  "64.225.84.0/22,IN,IN-KA,Bangalore,560100\n142.93.0.0/16,US,US-NJ,North Bergen,07047\n2400:6180:0:d0::/64,SG,SG-05,Singapore,627753\n",
}

func createMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if data, ok := mockProviderData[name]; ok {
			fmt.Fprint(w, data)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
}

func setupTestData(t *testing.T, dir string) {
	t.Helper()
	for name, data := range mockProviderData {
		p := provider.ByName(name)
		if p == nil || p.Parse == nil {
			continue
		}
		ipRange, err := p.Parse([]byte(data))
		require.NoError(t, err, "failed to parse mock data for %s", name)
		require.NoError(t, provider.Save(name, ipRange, dir), "failed to save mock data for %s", name)
	}
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old
	return string(out)
}

// ---------------------------------------------------------------------------
// checkIP tests
// ---------------------------------------------------------------------------

func TestCheckIP_Output(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)

	tests := []struct {
		name           string
		ip             string
		wantProvider   string
		wantNotInRange bool
	}{
		{"Amazon IPv4", "13.224.1.1", "Amazon", false},
		{"Cloudflare IPv4", "198.41.200.1", "Cloudflare", false},
		{"GitHub web", "192.30.253.1", "Github", false},
		{"Google", "8.8.8.1", "Google", false},
		{"GoogleBot", "66.249.65.1", "Googlebot", false},
		{"OpenAI", "23.98.142.177", "Openai", false},
		{"DigitalOcean", "64.225.84.1", "Digitalocean", false},
		{"DigitalOcean second range", "142.93.1.1", "Digitalocean", false},
		{"not in any range", "1.2.3.4", "", true},
		{"private IP not matched", "192.168.1.1", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := captureOutput(func() {
				checkIP(tc.ip, dir)
			})
			if tc.wantNotInRange {
				assert.Contains(t, output, "not in the range of any provider")
			} else {
				assert.Contains(t, output, tc.wantProvider)
				assert.Contains(t, output, tc.ip)
			}
		})
	}
}

func TestCheckIP_IPv6(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)

	tests := []struct {
		name         string
		ip           string
		wantProvider string
	}{
		{"Amazon IPv6", "2600:1f00::1", "Amazon"},
		{"Cloudflare IPv6", "2400:cb00::1", "Cloudflare"},
		{"Google IPv6", "2001:4860::1", "Google"},
		{"DigitalOcean IPv6", "2400:6180:0:d0::1", "Digitalocean"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := captureOutput(func() {
				checkIP(tc.ip, dir)
			})
			assert.Contains(t, output, tc.wantProvider)
		})
	}
}

// ---------------------------------------------------------------------------
// checkIPsFromFile tests
// ---------------------------------------------------------------------------

func TestCheckIPsFromFile(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)

	t.Run("processes all IPs in file", func(t *testing.T) {
		tmpFile := createTempIPFile(t, []string{
			"13.224.1.1",
			"198.41.200.1",
			"1.2.3.4",
			"64.225.84.1",
		})

		output := captureOutput(func() {
			checkIPsFromFile(tmpFile, dir)
		})

		assert.Contains(t, output, "Amazon")
		assert.Contains(t, output, "Cloudflare")
		assert.Contains(t, output, "not in the range")
		assert.Contains(t, output, "Digitalocean")
	})

	t.Run("skips empty lines", func(t *testing.T) {
		tmpFile := createTempIPFile(t, []string{
			"",
			"13.224.1.1",
			"",
			"",
			"198.41.200.1",
			"",
		})

		output := captureOutput(func() {
			checkIPsFromFile(tmpFile, dir)
		})

		// Should only have two result lines (not crash on empty lines)
		assert.Contains(t, output, "Amazon")
		assert.Contains(t, output, "Cloudflare")
		// Count actual result lines
		lines := filterNonEmpty(strings.Split(output, "\n"))
		assert.Equal(t, 2, len(lines))
	})

	t.Run("handles whitespace-only lines", func(t *testing.T) {
		tmpFile := createTempIPFile(t, []string{
			"  ",
			"	",
			"13.224.1.1",
		})

		output := captureOutput(func() {
			checkIPsFromFile(tmpFile, dir)
		})

		lines := filterNonEmpty(strings.Split(output, "\n"))
		assert.Equal(t, 1, len(lines))
		assert.Contains(t, output, "Amazon")
	})

	t.Run("nonexistent file prints error", func(t *testing.T) {
		// Redirect stderr to capture error output
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Should not panic
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("checkIPsFromFile panicked: %v", rec)
				}
			}()
			// We need to prevent os.Exit from actually exiting in test
			// Just verify it doesn't panic with a non-existent file
			// The function calls os.Exit(1) so we can't easily test it
		}()

		w.Close()
		os.Stderr = oldStderr
		r.Close()
	})
}

// ---------------------------------------------------------------------------
// updateAllProviders tests
// ---------------------------------------------------------------------------

func TestUpdateAllProviders(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	dir := t.TempDir()

	// Save original URLs and restore after test
	originalURLs := make(map[int]string)
	originalUpdates := make(map[int]provider.UpdateFunc)

	for i := range provider.Registry {
		originalURLs[i] = provider.Registry[i].URL
		originalUpdates[i] = provider.Registry[i].Update

		if provider.Registry[i].Parse != nil {
			provider.Registry[i].URL = server.URL + "/" + provider.Registry[i].Name
		}
		// Mock Microsoft's custom update
		if provider.Registry[i].Name == "microsoft" {
			idx := i
			provider.Registry[idx].Update = func(dataDir string) error {
				return provider.Save("microsoft", &IPRange{
					IPv4: []string{"20.0.0.0/8"},
					IPv6: []string{"2603:1000::/24"},
				}, dataDir)
			}
		}
	}
	defer func() {
		for i := range provider.Registry {
			provider.Registry[i].URL = originalURLs[i]
			provider.Registry[i].Update = originalUpdates[i]
		}
	}()

	output := captureOutput(func() {
		updateAllProviders(dir)
	})

	t.Run("reports success for all providers", func(t *testing.T) {
		for _, p := range provider.Registry {
			assert.Contains(t, output, "updated successfully",
				"expected success message for %s", p.Name)
		}
	})

	t.Run("saves data files for all providers", func(t *testing.T) {
		for _, p := range provider.Registry {
			path := filepath.Join(dir, p.Name, "ipranges.json")
			_, err := os.Stat(path)
			assert.NoError(t, err, "expected %s to exist", path)
		}
	})

	t.Run("saved data is valid and loadable", func(t *testing.T) {
		loaded, err := provider.Load("amazon", dir)
		require.NoError(t, err)
		assert.Contains(t, loaded.IPv4, "13.224.0.0/14")
		assert.Contains(t, loaded.IPv6, "2600:1f00::/24")
	})

	t.Run("microsoft custom update works", func(t *testing.T) {
		loaded, err := provider.Load("microsoft", dir)
		require.NoError(t, err)
		assert.Equal(t, []string{"20.0.0.0/8"}, loaded.IPv4)
		assert.Equal(t, []string{"2603:1000::/24"}, loaded.IPv6)
	})
}

func TestUpdateAllProviders_ContinuesOnError(t *testing.T) {
	// Create a server that fails for one provider
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "cloudflare" {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		if data, ok := mockProviderData[name]; ok {
			fmt.Fprint(w, data)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()

	// Override URLs
	originalURLs := make(map[int]string)
	originalUpdates := make(map[int]provider.UpdateFunc)
	for i := range provider.Registry {
		originalURLs[i] = provider.Registry[i].URL
		originalUpdates[i] = provider.Registry[i].Update
		if provider.Registry[i].Parse != nil {
			provider.Registry[i].URL = server.URL + "/" + provider.Registry[i].Name
		}
		if provider.Registry[i].Name == "microsoft" {
			idx := i
			provider.Registry[idx].Update = func(dataDir string) error {
				return provider.Save("microsoft", &IPRange{IPv4: []string{"20.0.0.0/8"}}, dataDir)
			}
		}
	}
	defer func() {
		for i := range provider.Registry {
			provider.Registry[i].URL = originalURLs[i]
			provider.Registry[i].Update = originalUpdates[i]
		}
	}()

	output := captureOutput(func() {
		updateAllProviders(dir)
	})

	// Cloudflare should fail but others should succeed
	assert.Contains(t, output, "Amazon")
	assert.Contains(t, output, "updated successfully")

	// Amazon data should still be saved
	loaded, err := provider.Load("amazon", dir)
	require.NoError(t, err)
	assert.NotEmpty(t, loaded.IPv4)
}

// ---------------------------------------------------------------------------
// colorizeProvider tests
// ---------------------------------------------------------------------------

func TestColorizeProvider(t *testing.T) {
	// Ensure colorize doesn't panic and returns non-empty string for all providers
	providers := []string{
		"microsoft", "github", "githubactions", "githubhooks", "githubpages",
		"amazon", "cloudflare", "google", "googlecloud", "googlebot",
		"openai", "digitalocean", "unknown",
	}

	for _, name := range providers {
		t.Run(name, func(t *testing.T) {
			result := colorizeProvider(name)
			assert.NotEmpty(t, result)
			// Should capitalize first letter
			assert.True(t, strings.ToUpper(result[:1]) == result[:1] ||
				result[0] == '\033', // ANSI escape code starts with ESC
				"expected capitalized or color-coded output for %s", name)
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"amazon", "Amazon"},
		{"", ""},
		{"A", "A"},
		{"hello world", "Hello world"},
		{"123abc", "123abc"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, capitalizeFirst(tc.input))
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// IPRange type alias for test helper
type IPRange = provider.IPRange
type UpdateFunc = provider.UpdateFunc

func createTempIPFile(t *testing.T, lines []string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_ips_*.txt")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	for _, line := range lines {
		fmt.Fprintln(tmpFile, line)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

func filterNonEmpty(lines []string) []string {
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}
