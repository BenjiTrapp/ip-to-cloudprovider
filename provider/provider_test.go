package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestRegistry(t *testing.T) {
	t.Run("all providers registered", func(t *testing.T) {
		expected := []string{
			"amazon", "cloudflare",
			"github", "githubactions", "githubhooks", "githubpages",
			"google", "googlecloud", "googlebot",
			"openai", "digitalocean", "microsoft",
			"alibaba", "anthropic", "hetzner",
		}
		names := Names()
		for _, name := range expected {
			assert.Contains(t, names, name)
		}
	})

	t.Run("ByName returns correct provider", func(t *testing.T) {
		p := ByName("amazon")
		require.NotNil(t, p)
		assert.Equal(t, "amazon", p.Name)
		assert.NotEmpty(t, p.URL)
		assert.NotNil(t, p.Parse)
	})

	t.Run("ByName returns nil for unknown", func(t *testing.T) {
		assert.Nil(t, ByName("nonexistent"))
	})
}

// ---------------------------------------------------------------------------
// Parser tests (table-driven)
// ---------------------------------------------------------------------------

func TestParseAmazon(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantV4  []string
		wantV6  []string
		wantErr bool
	}{
		{
			name: "valid data with both IPv4 and IPv6",
			input: `{
				"prefixes": [
					{"ip_prefix": "13.224.0.0/14"},
					{"ip_prefix": "52.94.76.0/22"}
				],
				"ipv6_prefixes": [
					{"ipv6_prefix": "2600:1f00::/24"}
				]
			}`,
			wantV4: []string{"13.224.0.0/14", "52.94.76.0/22"},
			wantV6: []string{"2600:1f00::/24"},
		},
		{
			name:   "empty prefixes",
			input:  `{"prefixes": [], "ipv6_prefixes": []}`,
			wantV4: nil,
			wantV6: nil,
		},
		{
			name:    "invalid JSON",
			input:   `{not valid json`,
			wantErr: true,
		},
		{
			name:   "missing ipv6_prefixes key",
			input:  `{"prefixes": [{"ip_prefix": "10.0.0.0/8"}]}`,
			wantV4: []string{"10.0.0.0/8"},
			wantV6: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAmazon([]byte(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseCloudflare(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantV4  []string
		wantV6  []string
		wantErr bool
	}{
		{
			name: "valid data",
			input: `{
				"result": {
					"ipv4_cidrs": ["198.41.128.0/17", "104.16.0.0/13"],
					"ipv6_cidrs": ["2400:cb00::/32"]
				}
			}`,
			wantV4: []string{"198.41.128.0/17", "104.16.0.0/13"},
			wantV6: []string{"2400:cb00::/32"},
		},
		{
			name:   "empty result",
			input:  `{"result": {"ipv4_cidrs": [], "ipv6_cidrs": []}}`,
			wantV4: []string{},
			wantV6: []string{},
		},
		{
			name:    "invalid JSON",
			input:   `broken`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseCloudflare([]byte(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseGitHub(t *testing.T) {
	fullMeta := `{
		"web": ["192.30.252.0/22", "2606:50c0::/32"],
		"actions": ["4.148.0.0/15", "2603:1030::/44"],
		"hooks": ["192.30.252.0/22"],
		"pages": ["185.199.108.0/22", "2606:50c0:8000::/48"]
	}`

	t.Run("web separates IPv4 and IPv6", func(t *testing.T) {
		result, err := parseGitHubWeb([]byte(fullMeta))
		require.NoError(t, err)
		assert.Equal(t, []string{"192.30.252.0/22"}, result.IPv4)
		assert.Equal(t, []string{"2606:50c0::/32"}, result.IPv6)
	})

	t.Run("actions separates IPv4 and IPv6", func(t *testing.T) {
		result, err := parseGitHubActions([]byte(fullMeta))
		require.NoError(t, err)
		assert.Equal(t, []string{"4.148.0.0/15"}, result.IPv4)
		assert.Equal(t, []string{"2603:1030::/44"}, result.IPv6)
	})

	t.Run("hooks IPv4 only", func(t *testing.T) {
		result, err := parseGitHubHooks([]byte(fullMeta))
		require.NoError(t, err)
		assert.Equal(t, []string{"192.30.252.0/22"}, result.IPv4)
		assert.Empty(t, result.IPv6)
	})

	t.Run("pages separates IPv4 and IPv6", func(t *testing.T) {
		result, err := parseGitHubPages([]byte(fullMeta))
		require.NoError(t, err)
		assert.Equal(t, []string{"185.199.108.0/22"}, result.IPv4)
		assert.Equal(t, []string{"2606:50c0:8000::/48"}, result.IPv6)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := parseGitHubWeb([]byte(`{invalid`))
		assert.Error(t, err)
	})
}

func TestParseGoogle(t *testing.T) {
	t.Run("txt format", func(t *testing.T) {
		tests := []struct {
			name   string
			input  string
			wantV4 []string
			wantV6 []string
		}{
			{
				name:   "mixed IPv4 and IPv6",
				input:  "8.8.4.0/24\n8.8.8.0/24\n2001:4860::/32\n",
				wantV4: []string{"8.8.4.0/24", "8.8.8.0/24"},
				wantV6: []string{"2001:4860::/32"},
			},
			{
				name:   "empty lines ignored",
				input:  "\n\n8.8.8.0/24\n\n",
				wantV4: []string{"8.8.8.0/24"},
				wantV6: nil,
			},
			{
				name:   "whitespace trimmed",
				input:  "  10.0.0.0/8  \n  2001:db8::/32  \n",
				wantV4: []string{"10.0.0.0/8"},
				wantV6: []string{"2001:db8::/32"},
			},
			{
				name:   "empty input",
				input:  "",
				wantV4: nil,
				wantV6: nil,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				result, err := parseGoogleTxt([]byte(tc.input))
				require.NoError(t, err)
				assert.Equal(t, tc.wantV4, result.IPv4)
				assert.Equal(t, tc.wantV6, result.IPv6)
			})
		}
	})

	t.Run("JSON format", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantV4  []string
			wantV6  []string
			wantErr bool
		}{
			{
				name: "mixed prefixes",
				input: `{"prefixes": [
					{"ipv4Prefix": "34.80.0.0/15"},
					{"ipv6Prefix": "2600:1900::/35"},
					{"ipv4Prefix": "35.220.0.0/14"}
				]}`,
				wantV4: []string{"34.80.0.0/15", "35.220.0.0/14"},
				wantV6: []string{"2600:1900::/35"},
			},
			{
				name:   "empty prefixes",
				input:  `{"prefixes": []}`,
				wantV4: nil,
				wantV6: nil,
			},
			{
				name:   "only ipv4",
				input:  `{"prefixes": [{"ipv4Prefix": "1.2.3.0/24"}]}`,
				wantV4: []string{"1.2.3.0/24"},
				wantV6: nil,
			},
			{
				name:    "invalid JSON",
				input:   `not json`,
				wantErr: true,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				result, err := parseGoogleJSON([]byte(tc.input))
				if tc.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.wantV4, result.IPv4)
				assert.Equal(t, tc.wantV6, result.IPv6)
			})
		}
	})
}

func TestParseOpenAI(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantV4 []string
		wantV6 []string
	}{
		{
			name:   "IPv4 only",
			input:  "23.98.142.176/28\n40.84.180.224/28\n",
			wantV4: []string{"23.98.142.176/28", "40.84.180.224/28"},
			wantV6: nil,
		},
		{
			name:   "mixed IPv4 and IPv6",
			input:  "23.98.142.176/28\n2607:f8b0:4000::/36\n",
			wantV4: []string{"23.98.142.176/28"},
			wantV6: []string{"2607:f8b0:4000::/36"},
		},
		{
			name:   "empty input",
			input:  "",
			wantV4: nil,
			wantV6: nil,
		},
		{
			name:   "trailing newlines and whitespace",
			input:  "\n  10.0.0.0/8  \n\n",
			wantV4: []string{"10.0.0.0/8"},
			wantV6: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseOpenAI([]byte(tc.input))
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseDigitalOcean(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantV4 []string
		wantV6 []string
	}{
		{
			name: "standard CSV rows",
			input: `5.101.96.0/21,NL,NL-NH,Amsterdam,1098 XH
24.144.64.0/22,US,US-NJ,North Bergen,07047
2400:6180:0:d0::/64,SG,SG-05,Singapore,627753
2604:a880:400:d0::/64,US,US-NJ,North Bergen,07047`,
			wantV4: []string{"5.101.96.0/21", "24.144.64.0/22"},
			wantV6: []string{"2400:6180:0:d0::/64", "2604:a880:400:d0::/64"},
		},
		{
			name:   "entries with None location",
			input:  "168.144.52.0/22,None,None,None,None\n",
			wantV4: []string{"168.144.52.0/22"},
			wantV6: nil,
		},
		{
			name:   "empty lines ignored",
			input:  "\n\n10.0.0.0/8,US,US-CA,SF,94000\n\n",
			wantV4: []string{"10.0.0.0/8"},
			wantV6: nil,
		},
		{
			name:   "empty input",
			input:  "",
			wantV4: nil,
			wantV6: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseDigitalOcean([]byte(tc.input))
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseAlibaba(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantV4 []string
		wantV6 []string
	}{
		{
			name: "standard format with comments",
			input: `# AS45102 (CNNIC-ALIBABA-CN-NET-AP)
# Alibaba (China) Technology Co. Ltd.
#
8.208.0.0/16
8.209.0.0/19
47.52.0.0/16
2400:3200::/48
2404:2280:1000::/36`,
			wantV4: []string{"8.208.0.0/16", "8.209.0.0/19", "47.52.0.0/16"},
			wantV6: []string{"2400:3200::/48", "2404:2280:1000::/36"},
		},
		{
			name:   "empty input",
			input:  "",
			wantV4: nil,
			wantV6: nil,
		},
		{
			name:   "only comments",
			input:  "# comment\n# another comment\n",
			wantV4: nil,
			wantV6: nil,
		},
		{
			name:   "whitespace and empty lines",
			input:  "\n\n  8.210.0.0/15  \n\n  2400:3200:baba::/48  \n",
			wantV4: []string{"8.210.0.0/15"},
			wantV6: []string{"2400:3200:baba::/48"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAlibaba([]byte(tc.input))
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseAnthropic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantV4  []string
		wantV6  []string
		wantErr bool
	}{
		{
			name: "extracts CIDRs from docs-like content",
			input: `IP addresses
Inbound IP addresses
IPv4
160.79.104.0/23
IPv6
2607:6bc0::/48
Outbound IP addresses
IPv4
160.79.104.0/21`,
			wantV4: []string{"160.79.104.0/23", "160.79.104.0/21"},
			wantV6: []string{"2607:6bc0::/48"},
		},
		{
			name: "excludes phased-out IPs",
			input: `Inbound IP addresses
IPv4
160.79.104.0/23
Phased out IP addresses
34.162.46.92/32
34.162.102.82/32`,
			wantV4: []string{"160.79.104.0/23"},
			wantV6: nil,
		},
		{
			name:    "no CIDRs found",
			input:   "No IP addresses here, just text.",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAnthropic([]byte(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestParseCommentedCIDRs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"skips comments", "# comment\n10.0.0.0/8\n# another\n", []string{"10.0.0.0/8"}},
		{"skips empty lines", "\n\n10.0.0.0/8\n\n", []string{"10.0.0.0/8"}},
		{"trims whitespace", "  10.0.0.0/8  \n", []string{"10.0.0.0/8"}},
		{"empty input", "", nil},
		{"only comments", "# just comments\n", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseCommentedCIDRs(tc.input))
		})
	}
}

func TestParseMicrosoftServiceTags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantV4  []string
		wantV6  []string
		wantErr bool
	}{
		{
			name: "deduplicates across services",
			input: `{
				"changeNumber": 123,
				"cloud": "Public",
				"values": [
					{
						"name": "ServiceA",
						"id": "ServiceA",
						"properties": {
							"addressPrefixes": ["13.65.0.0/16", "20.36.0.0/14", "2603:1030::/44"]
						}
					},
					{
						"name": "ServiceB",
						"id": "ServiceB",
						"properties": {
							"addressPrefixes": ["13.65.0.0/16", "40.74.0.0/15"]
						}
					}
				]
			}`,
			wantV4: []string{"13.65.0.0/16", "20.36.0.0/14", "40.74.0.0/15"},
			wantV6: []string{"2603:1030::/44"},
		},
		{
			name:   "empty values",
			input:  `{"changeNumber": 1, "cloud": "Public", "values": []}`,
			wantV4: nil,
			wantV6: nil,
		},
		{
			name: "handles whitespace in prefixes",
			input: `{
				"changeNumber": 1,
				"cloud": "Public",
				"values": [{
					"name": "Svc",
					"id": "Svc",
					"properties": {
						"addressPrefixes": ["  10.0.0.0/8  ", "2001:db8::/32"]
					}
				}]
			}`,
			wantV4: []string{"10.0.0.0/8"},
			wantV6: []string{"2001:db8::/32"},
		},
		{
			name:    "invalid JSON",
			input:   `{not valid`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := fetchAndParseMicrosoftServiceTagsFromBytes([]byte(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantV4, result.IPv4)
			assert.Equal(t, tc.wantV6, result.IPv6)
		})
	}
}

func TestDiscoverMicrosoftDownloadURL(t *testing.T) {
	t.Run("finds ServiceTags link in HTML", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<html><body>
				<a href="https://example.com/other.json">Other</a>
				<a href="https://download.microsoft.com/download/path/ServiceTags_Public_20240101.json">Download</a>
				<a href="https://example.com/another">Another</a>
			</body></html>`)
		}))
		defer server.Close()

		url, err := discoverMicrosoftDownloadURLFromPage(server.URL)
		require.NoError(t, err)
		assert.Equal(t, "https://download.microsoft.com/download/path/ServiceTags_Public_20240101.json", url)
	})

	t.Run("returns error when no link found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<html><body><a href="https://example.com">No tags here</a></body></html>`)
		}))
		defer server.Close()

		_, err := discoverMicrosoftDownloadURLFromPage(server.URL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no ServiceTags download link found")
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		_, err := discoverMicrosoftDownloadURLFromPage("http://127.0.0.1:1")
		assert.Error(t, err)
	})
}

func TestMicrosoftEndToEnd(t *testing.T) {
	// Mock the ServiceTags JSON download
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"changeNumber": 1,
			"cloud": "Public",
			"values": [{
				"name": "TestSvc",
				"id": "TestSvc",
				"properties": {
					"addressPrefixes": ["10.0.0.0/8", "2001:db8::/32"]
				}
			}]
		}`)
	}))
	defer downloadServer.Close()

	// Mock the confirmation page that points to the download server
	// The link must contain "download.microsoft.com" and "ServiceTags" to match the scraper
	confirmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use a fake download.microsoft.com URL that actually redirects to our mock
		fmt.Fprintf(w, `<html><body>
			<a href="https://download.microsoft.com/download/path/ServiceTags_Public.json">Download</a>
		</body></html>`)
	}))
	defer confirmServer.Close()

	// Test the URL discovery from the confirmation page
	url, err := discoverMicrosoftDownloadURLFromPage(confirmServer.URL)
	require.NoError(t, err)
	assert.Contains(t, url, "download.microsoft.com")
	assert.Contains(t, url, "ServiceTags")

	// Test parsing the actual ServiceTags JSON (using our download mock directly)
	result, err := fetchAndParseMicrosoftServiceTags(downloadServer.URL)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.0/8"}, result.IPv4)
	assert.Equal(t, []string{"2001:db8::/32"}, result.IPv6)
}

// ---------------------------------------------------------------------------
// IsIPInRange tests
// ---------------------------------------------------------------------------

func TestIsIPInRange(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		ranges []string
		want   bool
	}{
		{"IPv4 in range", "192.168.1.100", []string{"192.168.1.0/24"}, true},
		{"IPv4 in large range", "10.1.2.3", []string{"10.0.0.0/8"}, true},
		{"IPv4 not in range", "172.16.0.1", []string{"192.168.1.0/24", "10.0.0.0/8"}, false},
		{"IPv4 boundary start", "192.168.1.0", []string{"192.168.1.0/24"}, true},
		{"IPv4 boundary end", "192.168.1.255", []string{"192.168.1.0/24"}, true},
		{"IPv4 just outside", "192.168.2.0", []string{"192.168.1.0/24"}, false},
		{"IPv6 in range", "2001:db8::1", []string{"2001:db8::/32"}, true},
		{"IPv6 not in range", "2001:db9::1", []string{"2001:db8::/32"}, false},
		{"invalid IP returns false", "not-an-ip", []string{"10.0.0.0/8"}, false},
		{"empty IP returns false", "", []string{"10.0.0.0/8"}, false},
		{"empty ranges returns false", "10.0.0.1", []string{}, false},
		{"nil ranges returns false", "10.0.0.1", nil, false},
		{"invalid CIDR skipped", "10.0.0.1", []string{"invalid", "10.0.0.0/8"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsIPInRange(tc.ip, tc.ranges))
		})
	}
}

// ---------------------------------------------------------------------------
// ClassifyCIDR tests
// ---------------------------------------------------------------------------

func TestClassifyCIDR(t *testing.T) {
	tests := []struct {
		cidr   string
		isIPv6 bool
	}{
		{"10.0.0.0/8", false},
		{"192.168.0.0/16", false},
		{"2001:db8::/32", true},
		{"::1/128", true},
		{"2400:cb00::/32", true},
		{"fe80::/10", true},
	}

	for _, tc := range tests {
		t.Run(tc.cidr, func(t *testing.T) {
			assert.Equal(t, tc.isIPv6, ClassifyCIDR(tc.cidr))
		})
	}
}

// ---------------------------------------------------------------------------
// Save / Load tests
// ---------------------------------------------------------------------------

func TestSaveAndLoad(t *testing.T) {
	t.Run("round-trip preserves data", func(t *testing.T) {
		dir := t.TempDir()
		ipRange := &IPRange{
			IPv4: []string{"10.0.0.0/8", "192.168.0.0/16"},
			IPv6: []string{"2001:db8::/32"},
		}

		err := Save("testprovider", ipRange, dir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dir, "testprovider", "ipranges.json"))
		require.NoError(t, err)

		loaded, err := Load("testprovider", dir)
		require.NoError(t, err)
		assert.Equal(t, ipRange, loaded)
	})

	t.Run("creates directory if missing", func(t *testing.T) {
		dir := t.TempDir()
		ipRange := &IPRange{IPv4: []string{"1.2.3.0/24"}}

		err := Save("newdir", ipRange, dir)
		require.NoError(t, err)

		info, err := os.Stat(filepath.Join(dir, "newdir"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("load returns error for missing file", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load("nonexistent", dir)
		assert.Error(t, err)
	})

	t.Run("load returns error for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		provDir := filepath.Join(dir, "broken")
		require.NoError(t, os.MkdirAll(provDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(provDir, "ipranges.json"), []byte("{not json"), 0644))

		_, err := Load("broken", dir)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// FetchAndParse tests
// ---------------------------------------------------------------------------

func TestFetchAndParse(t *testing.T) {
	t.Run("successful fetch and parse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"prefixes": [{"ip_prefix": "13.224.0.0/14"}],
				"ipv6_prefixes": [{"ipv6_prefix": "2600:1f00::/24"}]
			}`))
		}))
		defer server.Close()

		p := &Provider{Name: "test", URL: server.URL, Parse: parseAmazon}
		result, err := FetchAndParse(p)
		require.NoError(t, err)
		assert.Equal(t, []string{"13.224.0.0/14"}, result.IPv4)
		assert.Equal(t, []string{"2600:1f00::/24"}, result.IPv6)
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}))
		defer server.Close()

		p := &Provider{Name: "test", URL: server.URL, Parse: parseAmazon}
		_, err := FetchAndParse(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
	})

	t.Run("returns error on connection failure", func(t *testing.T) {
		p := &Provider{Name: "test", URL: "http://127.0.0.1:1", Parse: parseAmazon}
		_, err := FetchAndParse(p)
		assert.Error(t, err)
	})

	t.Run("returns error when parser is nil", func(t *testing.T) {
		p := &Provider{Name: "test", URL: "http://example.com"}
		_, err := FetchAndParse(p)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no parser")
	})

	t.Run("returns error on parse failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not valid json`))
		}))
		defer server.Close()

		p := &Provider{Name: "test", URL: server.URL, Parse: parseAmazon}
		_, err := FetchAndParse(p)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// UpdateProvider tests
// ---------------------------------------------------------------------------

func TestUpdateProvider(t *testing.T) {
	t.Run("uses custom Update function when set", func(t *testing.T) {
		dir := t.TempDir()
		called := false

		p := &Provider{
			Name: "custom",
			Update: func(dataDir string) error {
				called = true
				return Save("custom", &IPRange{IPv4: []string{"1.1.1.0/24"}}, dataDir)
			},
		}

		err := UpdateProvider(p, dir)
		require.NoError(t, err)
		assert.True(t, called)

		loaded, err := Load("custom", dir)
		require.NoError(t, err)
		assert.Equal(t, []string{"1.1.1.0/24"}, loaded.IPv4)
	})

	t.Run("uses URL+Parse when no Update function", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "10.0.0.0/8\n2001:db8::/32\n")
		}))
		defer server.Close()

		dir := t.TempDir()
		p := &Provider{Name: "testprov", URL: server.URL, Parse: parseOpenAI}

		err := UpdateProvider(p, dir)
		require.NoError(t, err)

		loaded, err := Load("testprov", dir)
		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8"}, loaded.IPv4)
		assert.Equal(t, []string{"2001:db8::/32"}, loaded.IPv6)
	})
}

// ---------------------------------------------------------------------------
// CheckIP tests
// ---------------------------------------------------------------------------

func TestCheckIP(t *testing.T) {
	dir := t.TempDir()

	// Save test data for multiple providers
	require.NoError(t, Save("amazon", &IPRange{
		IPv4: []string{"13.224.0.0/14"},
		IPv6: []string{"2600:1f00::/24"},
	}, dir))
	require.NoError(t, Save("cloudflare", &IPRange{
		IPv4: []string{"198.41.128.0/17"},
		IPv6: []string{"2400:cb00::/32"},
	}, dir))

	tests := []struct {
		name     string
		ip       string
		expected string
	}{
		{"IPv4 in Amazon", "13.224.1.1", "amazon"},
		{"IPv4 in Cloudflare", "198.41.200.1", "cloudflare"},
		{"IPv6 in Amazon", "2600:1f00::1", "amazon"},
		{"IPv6 in Cloudflare", "2400:cb00::1", "cloudflare"},
		{"not in any provider", "1.2.3.4", ""},
		{"invalid IP", "not-an-ip", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, CheckIP(tc.ip, dir))
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkIsIPInRange_SmallSet(b *testing.B) {
	ranges := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsIPInRange("192.168.1.100", ranges)
	}
}

func BenchmarkIsIPInRange_LargeSet(b *testing.B) {
	// Simulate a large provider with many ranges
	ranges := make([]string, 1000)
	for i := range ranges {
		ranges[i] = fmt.Sprintf("%d.%d.0.0/16", i/256, i%256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsIPInRange("200.100.50.1", ranges)
	}
}

func BenchmarkCheckIP(b *testing.B) {
	dir := b.TempDir()

	// Set up data for a few providers
	Save("amazon", &IPRange{IPv4: generateRanges(100)}, dir)
	Save("cloudflare", &IPRange{IPv4: generateRanges(50)}, dir)
	Save("google", &IPRange{IPv4: generateRanges(200)}, dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckIP("200.100.50.1", dir)
	}
}

func generateRanges(n int) []string {
	ranges := make([]string, n)
	for i := range ranges {
		ranges[i] = fmt.Sprintf("%d.%d.0.0/16", (i*3)%256, (i*7)%256)
	}
	return ranges
}
