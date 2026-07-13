package shodan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a client pointed at the given base URL.
func newTestClient(baseURL string) *Client {
	c := NewClient("test-key")
	c.baseURL = baseURL
	return c
}

func TestScan_IP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasPrefix(r.URL.Path, "/shodan/host/"), "unexpected path %s", r.URL.Path)
		assert.Equal(t, "test-key", r.URL.Query().Get("key"))
		w.Write([]byte(`{
			"ip_str": "8.8.8.8",
			"ports": [443, 53],
			"hostnames": ["dns.google"],
			"org": "Google LLC",
			"country_name": "United States",
			"city": "Mountain View",
			"tags": ["cloud"],
			"vulns": ["CVE-2021-1234"],
			"data": [
				{"port": 53, "transport": "udp", "product": "Google DNS"},
				{"port": 443, "transport": "tcp", "product": "nginx", "version": "1.2"}
			]
		}`))
	}))
	defer srv.Close()

	res := newTestClient(srv.URL).Scan(context.Background(), "8.8.8.8")
	require.Empty(t, res.Err)
	require.False(t, res.IsDomain)
	require.NotNil(t, res.Host)

	h := res.Host
	assert.Equal(t, "8.8.8.8", h.IP)
	assert.Equal(t, []int{53, 443}, h.Ports) // sorted
	assert.Equal(t, "Google LLC", h.Org)
	assert.Equal(t, []string{"CVE-2021-1234"}, h.Vulns)
	require.Len(t, h.Services, 2)
	assert.Equal(t, 53, h.Services[0].Port) // sorted by port
	assert.Equal(t, "443/tcp nginx 1.2", h.Services[1].String())
}

func TestScan_Domain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/dns/resolve"):
			assert.Equal(t, "example.com", r.URL.Query().Get("hostnames"))
			w.Write([]byte(`{"example.com": "93.184.216.34"}`))
		case strings.HasPrefix(r.URL.Path, "/shodan/host/"):
			assert.Contains(t, r.URL.Path, "93.184.216.34")
			w.Write([]byte(`{"ip_str": "93.184.216.34", "ports": [80]}`))
		default:
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res := newTestClient(srv.URL).Scan(context.Background(), "example.com")
	require.Empty(t, res.Err)
	assert.True(t, res.IsDomain)
	assert.Equal(t, "93.184.216.34", res.ResolvedIP)
	require.NotNil(t, res.Host)
	assert.Equal(t, []int{80}, res.Host.Ports)
}

func TestScan_HostError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "No information available for that IP."}`))
	}))
	defer srv.Close()

	res := newTestClient(srv.URL).Scan(context.Background(), "1.2.3.4")
	assert.Nil(t, res.Host)
	assert.Equal(t, "No information available for that IP.", res.Err)
}

func TestScan_DomainResolveError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Resolve returns an empty mapping -> cannot resolve.
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	res := newTestClient(srv.URL).Scan(context.Background(), "nonexistent.invalid")
	assert.True(t, res.IsDomain)
	assert.Nil(t, res.Host)
	assert.Contains(t, res.Err, "could not resolve")
}

func TestServiceString(t *testing.T) {
	assert.Equal(t, "22", Service{Port: 22}.String())
	assert.Equal(t, "22/tcp", Service{Port: 22, Transport: "tcp"}.String())
	assert.Equal(t, "22/tcp OpenSSH 8.9", Service{Port: 22, Transport: "tcp", Product: "OpenSSH", Version: "8.9"}.String())
}

func TestConfig_EnvKeyFallback(t *testing.T) {
	t.Setenv(apiKeyEnv, "env-key")
	cfg := DefaultConfig()
	assert.Equal(t, "env-key", cfg.APIKey())
}

func TestLoadConfig_FromFile(t *testing.T) {
	t.Setenv(apiKeyEnv, "")
	path := filepath.Join(t.TempDir(), "reputation.yaml")
	content := "shodan:\n  enabled: true\n  api_key: file-key\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "file-key", cfg.APIKey())
	assert.True(t, cfg.Shodan.Enabled)
}

func TestLoadConfig_MissingFileNoKey(t *testing.T) {
	t.Setenv(apiKeyEnv, "")
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	assert.Empty(t, cfg.APIKey())
}

func TestLoadConfig_FileKeyBeatsEnv(t *testing.T) {
	t.Setenv(apiKeyEnv, "env-key")
	path := filepath.Join(t.TempDir(), "reputation.yaml")
	require.NoError(t, os.WriteFile(path, []byte("shodan:\n  api_key: file-key\n"), 0644))

	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "file-key", cfg.APIKey()) // file key present, env not used
}
