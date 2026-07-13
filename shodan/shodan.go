// Package shodan provides a small client for the Shodan REST API and a scan
// helper that enriches an IP or domain with the data Shodan holds about it:
// open ports, detected services, tags, and known vulnerabilities.
//
// Domains are resolved to an IP via Shodan's DNS endpoint before the host
// lookup. The API key is supplied via the config file (see config.go) or the
// SHODAN_API_KEY environment variable.
package shodan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// apiBase is the Shodan REST API root.
	apiBase = "https://api.shodan.io"

	// httpTimeout bounds a single API request.
	httpTimeout = 15 * time.Second

	// maxResponseSize caps a response body to avoid unbounded reads (10 MB).
	maxResponseSize = 10 << 20
)

// Client talks to the Shodan REST API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient builds a Shodan client for the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: apiBase,
		http:    &http.Client{Timeout: httpTimeout},
	}
}

// Service is a single service Shodan observed on a host.
type Service struct {
	Port      int    `json:"port"`
	Transport string `json:"transport,omitempty"`
	Product   string `json:"product,omitempty"`
	Version   string `json:"version,omitempty"`
}

// String renders a service as "port/transport product version".
func (s Service) String() string {
	out := fmt.Sprintf("%d", s.Port)
	if s.Transport != "" {
		out += "/" + s.Transport
	}
	if s.Product != "" {
		out += " " + s.Product
	}
	if s.Version != "" {
		out += " " + s.Version
	}
	return out
}

// HostInfo is the subset of Shodan host data we surface.
type HostInfo struct {
	IP         string    `json:"ip"`
	Hostnames  []string  `json:"hostnames,omitempty"`
	Ports      []int     `json:"ports,omitempty"`
	Org        string    `json:"org,omitempty"`
	ISP        string    `json:"isp,omitempty"`
	OS         string    `json:"os,omitempty"`
	Country    string    `json:"country,omitempty"`
	City       string    `json:"city,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Vulns      []string  `json:"vulns,omitempty"`
	Services   []Service `json:"services,omitempty"`
	LastUpdate string    `json:"last_update,omitempty"`
}

// Result is the outcome of scanning a single target (IP or domain).
type Result struct {
	Target     string    `json:"target"`
	IsDomain   bool      `json:"is_domain"`
	ResolvedIP string    `json:"resolved_ip,omitempty"`
	Host       *HostInfo `json:"host,omitempty"`
	Err        string    `json:"error,omitempty"`
}

// hostResponse mirrors the Shodan /shodan/host/{ip} response fields we consume.
type hostResponse struct {
	IPStr      string   `json:"ip_str"`
	Ports      []int    `json:"ports"`
	Hostnames  []string `json:"hostnames"`
	Org        string   `json:"org"`
	ISP        string   `json:"isp"`
	OS         string   `json:"os"`
	Country    string   `json:"country_name"`
	City       string   `json:"city"`
	Tags       []string `json:"tags"`
	Vulns      []string `json:"vulns"`
	LastUpdate string   `json:"last_update"`
	Data       []struct {
		Port      int    `json:"port"`
		Transport string `json:"transport"`
		Product   string `json:"product"`
		Version   string `json:"version"`
	} `json:"data"`
	Error string `json:"error"`
}

// Scan looks up a target on Shodan. If the target is not an IP literal, it is
// treated as a domain and resolved to an IP first. Failures are returned in
// Result.Err rather than as a Go error, so batch callers can keep going.
func (c *Client) Scan(ctx context.Context, target string) Result {
	res := Result{Target: target}

	ip := target
	if net.ParseIP(target) == nil {
		res.IsDomain = true
		resolved, err := c.Resolve(ctx, target)
		if err != nil {
			res.Err = err.Error()
			return res
		}
		res.ResolvedIP = resolved
		ip = resolved
	}

	host, err := c.HostInfo(ctx, ip)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Host = host
	return res
}

// Resolve maps a domain to an IP address using Shodan's DNS endpoint.
func (c *Client) Resolve(ctx context.Context, domain string) (string, error) {
	endpoint := fmt.Sprintf("%s/dns/resolve?hostnames=%s&key=%s",
		c.baseURL, url.QueryEscape(domain), url.QueryEscape(c.apiKey))

	body, status, err := c.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("resolving %s: %s", domain, apiError(body, status))
	}

	var resolved map[string]string
	if err := json.Unmarshal(body, &resolved); err != nil {
		return "", fmt.Errorf("decoding resolve response: %w", err)
	}

	ip, ok := resolved[domain]
	if !ok || ip == "" {
		return "", fmt.Errorf("could not resolve %s", domain)
	}
	return ip, nil
}

// HostInfo fetches Shodan's host record for an IP.
func (c *Client) HostInfo(ctx context.Context, ip string) (*HostInfo, error) {
	endpoint := fmt.Sprintf("%s/shodan/host/%s?key=%s",
		c.baseURL, url.PathEscape(ip), url.QueryEscape(c.apiKey))

	body, status, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var parsed hostResponse
	if jsonErr := json.Unmarshal(body, &parsed); jsonErr != nil && status == http.StatusOK {
		return nil, fmt.Errorf("decoding host response: %w", jsonErr)
	}

	if status != http.StatusOK {
		if parsed.Error != "" {
			return nil, fmt.Errorf("%s", parsed.Error)
		}
		return nil, fmt.Errorf("%s", apiError(body, status))
	}

	info := &HostInfo{
		IP:         parsed.IPStr,
		Hostnames:  parsed.Hostnames,
		Ports:      parsed.Ports,
		Org:        parsed.Org,
		ISP:        parsed.ISP,
		OS:         parsed.OS,
		Country:    parsed.Country,
		City:       parsed.City,
		Tags:       parsed.Tags,
		Vulns:      parsed.Vulns,
		LastUpdate: parsed.LastUpdate,
	}
	if info.IP == "" {
		info.IP = ip
	}
	for _, d := range parsed.Data {
		info.Services = append(info.Services, Service{
			Port:      d.Port,
			Transport: d.Transport,
			Product:   d.Product,
			Version:   d.Version,
		})
	}
	sort.Ints(info.Ports)
	sort.Slice(info.Services, func(i, j int) bool {
		return info.Services[i].Port < info.Services[j].Port
	})
	sort.Strings(info.Vulns)

	return info, nil
}

// get performs a GET request and returns the (size-limited) body and status.
func (c *Client) get(ctx context.Context, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// apiError extracts a Shodan error message from a response body, falling back
// to the HTTP status when no structured error is present.
func apiError(body []byte, status int) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	msg := strings.TrimSpace(string(body))
	if msg != "" && len(msg) < 200 {
		return msg
	}
	return fmt.Sprintf("HTTP %d", status)
}
