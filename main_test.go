package main

import (
	"encoding/json"
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

func withDataDir(t *testing.T, dir string) func() {
	t.Helper()
	orig := dataDir
	dataDir = dir
	origQuiet := quiet
	quiet = true
	origJSON := jsonOutput
	return func() {
		dataDir = orig
		quiet = origQuiet
		jsonOutput = origJSON
	}
}

// ---------------------------------------------------------------------------
// scanIPs tests
// ---------------------------------------------------------------------------

func TestScanIPs_TextOutput(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)
	defer withDataDir(t, dir)()
	jsonOutput = false

	tests := []struct {
		name         string
		ips          []string
		wantContains []string
	}{
		{"single Amazon IP", []string{"13.224.1.1"}, []string{"13.224.1.1", "Amazon"}},
		{"multiple IPs", []string{"13.224.1.1", "198.41.200.1", "1.2.3.4"}, []string{"Amazon", "Cloudflare", "not in the range"}},
		{"IPv6 lookup", []string{"2400:cb00::1"}, []string{"Cloudflare"}},
		{"DigitalOcean", []string{"64.225.84.1"}, []string{"Digitalocean"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := captureOutput(func() { scanIPs(tc.ips) })
			for _, want := range tc.wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestScanIPs_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)
	defer withDataDir(t, dir)()
	jsonOutput = true

	output := captureOutput(func() {
		scanIPs([]string{"13.224.1.1", "198.41.200.1", "1.2.3.4"})
	})

	var results []provider.MatchResult
	require.NoError(t, json.Unmarshal([]byte(output), &results))
	require.Len(t, results, 3)

	assert.Equal(t, "amazon", results[0].Provider)
	assert.True(t, results[0].Match)
	assert.Equal(t, "cloudflare", results[1].Provider)
	assert.True(t, results[1].Match)
	assert.Equal(t, "", results[2].Provider)
	assert.False(t, results[2].Match)
}

func TestScanIPs_Stats(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)
	defer withDataDir(t, dir)()
	jsonOutput = false
	showStats = true
	defer func() { showStats = false }()

	output := captureOutput(func() {
		scanIPs([]string{"13.224.1.1", "13.224.2.2", "198.41.200.1", "1.2.3.4"})
	})

	assert.Contains(t, output, "Summary")
	assert.Contains(t, output, "4 IPs scanned")
}

func TestScanIPs_NoDataWarning(t *testing.T) {
	dir := t.TempDir() // empty dir, no provider data
	defer withDataDir(t, dir)()
	jsonOutput = false

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	captureOutput(func() { scanIPs([]string{"1.2.3.4"}) })

	w.Close()
	errOut, _ := io.ReadAll(r)
	os.Stderr = oldStderr

	assert.Contains(t, string(errOut), "no provider data found")
}

// ---------------------------------------------------------------------------
// readIPsFromFile tests
// ---------------------------------------------------------------------------

func TestReadIPsFromFile(t *testing.T) {
	t.Run("reads all IPs", func(t *testing.T) {
		f := createTempIPFile(t, []string{"1.2.3.4", "5.6.7.8"})
		assert.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, readIPsFromFile(f))
	})

	t.Run("skips empty and whitespace lines", func(t *testing.T) {
		f := createTempIPFile(t, []string{"", "  ", "1.2.3.4", "", "5.6.7.8"})
		assert.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, readIPsFromFile(f))
	})
}

// ---------------------------------------------------------------------------
// collectIPs tests
// ---------------------------------------------------------------------------

func TestCollectIPs(t *testing.T) {
	t.Run("from args", func(t *testing.T) {
		ips := collectIPs([]string{"1.2.3.4", "  5.6.7.8  ", ""}, "")
		assert.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, ips)
	})

	t.Run("from file", func(t *testing.T) {
		f := createTempIPFile(t, []string{"10.0.0.1", "10.0.0.2"})
		ips := collectIPs([]string{"1.2.3.4"}, f)
		assert.Equal(t, []string{"1.2.3.4", "10.0.0.1", "10.0.0.2"}, ips)
	})
}

// ---------------------------------------------------------------------------
// updateAllProviders tests
// ---------------------------------------------------------------------------

func TestUpdateAllProviders(t *testing.T) {
	server := createMockServer()
	defer server.Close()

	dir := t.TempDir()
	defer withDataDir(t, dir)()

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

	output := captureOutput(func() { updateAllProviders() })

	assert.Contains(t, output, "updated successfully")

	// All providers should have data files
	for _, p := range provider.Registry {
		path := filepath.Join(dir, p.Name, "ipranges.json")
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected %s to exist", path)
	}
}

// ---------------------------------------------------------------------------
// listProviders tests
// ---------------------------------------------------------------------------

func TestListProviders(t *testing.T) {
	dir := t.TempDir()
	setupTestData(t, dir)
	defer withDataDir(t, dir)()

	t.Run("text output", func(t *testing.T) {
		jsonOutput = false
		output := captureOutput(func() { listProviders() })
		assert.Contains(t, output, "PROVIDER")
		assert.Contains(t, output, "ready")
		assert.Contains(t, output, "providers registered")
	})

	t.Run("json output", func(t *testing.T) {
		jsonOutput = true
		output := captureOutput(func() { listProviders() })
		var infos []map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(output), &infos))
		assert.NotEmpty(t, infos)
		assert.Contains(t, infos[0], "name")
		assert.Contains(t, infos[0], "has_data")
	})
}

// ---------------------------------------------------------------------------
// Utility tests
// ---------------------------------------------------------------------------

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"amazon", "Amazon"}, {"", ""}, {"A", "A"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, capitalizeFirst(tc.input))
	}
}

// Type aliases for test helpers
type IPRange = provider.IPRange
type UpdateFunc = provider.UpdateFunc
