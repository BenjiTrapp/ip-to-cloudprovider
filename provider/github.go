package provider

import (
	"encoding/json"
	"fmt"
)

const gitHubMetaURL = "https://api.github.com/meta"

func init() {
	Register(Provider{
		Name:  "github",
		URL:   gitHubMetaURL,
		Parse: parseGitHubWeb,
	})
	Register(Provider{
		Name:  "githubactions",
		URL:   gitHubMetaURL,
		Parse: parseGitHubActions,
	})
	Register(Provider{
		Name:  "githubhooks",
		URL:   gitHubMetaURL,
		Parse: parseGitHubHooks,
	})
	Register(Provider{
		Name:  "githubpages",
		URL:   gitHubMetaURL,
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

// UpdateGitHubAll fetches the GitHub /meta endpoint once and saves all
// sub-providers, avoiding redundant HTTP requests.
func UpdateGitHubAll(dataDir string) error {
	body, err := Fetch(gitHubMetaURL)
	if err != nil {
		return fmt.Errorf("fetching GitHub meta: %w", err)
	}

	parsers := map[string]ParseFunc{
		"github":        parseGitHubWeb,
		"githubactions": parseGitHubActions,
		"githubhooks":   parseGitHubHooks,
		"githubpages":   parseGitHubPages,
	}

	for name, parse := range parsers {
		ipRange, err := parse(body)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", name, err)
		}
		if err := Save(name, ipRange, dataDir); err != nil {
			return fmt.Errorf("saving %s: %w", name, err)
		}
	}

	return nil
}

// IsGitHubProvider returns true if the provider is one of the GitHub sub-providers.
func IsGitHubProvider(name string) bool {
	switch name {
	case "github", "githubactions", "githubhooks", "githubpages":
		return true
	}
	return false
}
