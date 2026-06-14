package provider

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register(Provider{
		Name:  "github",
		URL:   "https://api.github.com/meta",
		Parse: parseGitHubWeb,
	})
	Register(Provider{
		Name:  "githubactions",
		URL:   "https://api.github.com/meta",
		Parse: parseGitHubActions,
	})
	Register(Provider{
		Name:  "githubhooks",
		URL:   "https://api.github.com/meta",
		Parse: parseGitHubHooks,
	})
	Register(Provider{
		Name:  "githubpages",
		URL:   "https://api.github.com/meta",
		Parse: parseGitHubPages,
	})
}

// gitHubMeta represents the GitHub /meta API response.
type gitHubMeta struct {
	Web     []string `json:"web"`
	Actions []string `json:"actions"`
	Hooks   []string `json:"hooks"`
	Pages   []string `json:"pages"`
}

func parseGitHubMeta(data []byte) (*gitHubMeta, error) {
	var meta gitHubMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing GitHub meta: %w", err)
	}
	return &meta, nil
}

func splitIPv4v6(cidrs []string) *IPRange {
	ipRange := &IPRange{}
	for _, cidr := range cidrs {
		if ClassifyCIDR(cidr) {
			ipRange.IPv6 = append(ipRange.IPv6, cidr)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, cidr)
		}
	}
	return ipRange
}

func parseGitHubWeb(data []byte) (*IPRange, error) {
	meta, err := parseGitHubMeta(data)
	if err != nil {
		return nil, err
	}
	return splitIPv4v6(meta.Web), nil
}

func parseGitHubActions(data []byte) (*IPRange, error) {
	meta, err := parseGitHubMeta(data)
	if err != nil {
		return nil, err
	}
	return splitIPv4v6(meta.Actions), nil
}

func parseGitHubHooks(data []byte) (*IPRange, error) {
	meta, err := parseGitHubMeta(data)
	if err != nil {
		return nil, err
	}
	return splitIPv4v6(meta.Hooks), nil
}

func parseGitHubPages(data []byte) (*IPRange, error) {
	meta, err := parseGitHubMeta(data)
	if err != nil {
		return nil, err
	}
	return splitIPv4v6(meta.Pages), nil
}
