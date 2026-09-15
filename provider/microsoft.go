package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func init() {
	Register(Provider{
		Name:   "microsoft",
		URL:    "", // Microsoft requires multi-step fetching
		Update: updateMicrosoft,
	})
}

// microsoftDownloadIDs maps Azure cloud names to their Microsoft download IDs.
// IDs are per the Azure "service tags overview" docs (Discover service tags by
// using downloadable JSON files).
// Note: Germany (57064) was retired Oct 2021 but still serves its final data.
var microsoftDownloadIDs = []struct {
	Cloud    string
	ID       string
	Required bool // if false, failure is non-fatal
}{
	{"Public", "56519", true},
	{"USGov", "57063", true},
	{"China", "57062", false},   // sometimes blocked from outside China
	{"Germany", "57064", false}, // retired Oct 2021, serves stale data
}

// serviceTagsFile represents the structure of Microsoft's ServiceTags JSON.
type serviceTagsFile struct {
	ChangeNumber int               `json:"changeNumber"`
	Cloud        string            `json:"cloud"`
	Values       []serviceTagValue `json:"values"`
}

type serviceTagValue struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Properties struct {
		AddressPrefixes []string `json:"addressPrefixes"`
	} `json:"properties"`
}

// updateMicrosoft fetches IP ranges from all Azure clouds and merges them.
// Required clouds (Public, USGov) must succeed; optional clouds (China, Germany)
// are best-effort and log errors without failing the entire update.
func updateMicrosoft(dataDir string) error {
	ipRange := &IPRange{}
	seen := make(map[string]bool)
	successCount := 0

	for _, cloud := range microsoftDownloadIDs {
		ranges, err := fetchMicrosoftCloud(cloud.ID)
		if err != nil {
			if cloud.Required {
				return fmt.Errorf("fetching Azure %s (id=%s): %w", cloud.Cloud, cloud.ID, err)
			}
			// Non-fatal: skip optional clouds that fail
			continue
		}

		// Merge and deduplicate
		for _, cidr := range ranges.IPv4 {
			if !seen[cidr] {
				seen[cidr] = true
				ipRange.IPv4 = append(ipRange.IPv4, cidr)
			}
		}
		for _, cidr := range ranges.IPv6 {
			if !seen[cidr] {
				seen[cidr] = true
				ipRange.IPv6 = append(ipRange.IPv6, cidr)
			}
		}
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("all Azure cloud fetches failed")
	}

	return Save("microsoft", ipRange, dataDir)
}

// fetchMicrosoftCloud discovers the download link for one Azure cloud (by its
// Microsoft download ID) and downloads + parses its ServiceTags JSON. It is a
// package variable so tests can stub the network layer and exercise the
// merge/dedup and required-vs-optional logic in updateMicrosoft directly.
var fetchMicrosoftCloud = func(id string) (*IPRange, error) {
	downloadURL, err := discoverMicrosoftDownloadURL(id)
	if err != nil {
		return nil, err
	}
	return fetchAndParseMicrosoftServiceTags(downloadURL)
}

// microsoftPageURL builds the download details page URL for a given download ID.
// It is a package variable so tests can redirect discovery to a mock server.
// The legacy confirmation.aspx endpoint was retired (it now redirects to a 404
// page), so details.aspx is used instead.
var microsoftPageURL = func(id string) string {
	return fmt.Sprintf("https://www.microsoft.com/download/details.aspx?id=%s", id)
}

// discoverMicrosoftDownloadURL scrapes the Microsoft download details page
// to find the actual JSON download link.
func discoverMicrosoftDownloadURL(id string) (string, error) {
	return discoverMicrosoftDownloadURLFromPage(microsoftPageURL(id))
}

// discoverMicrosoftDownloadURLFromPage fetches the given page URL and extracts
// the ServiceTags download link from the HTML.
func discoverMicrosoftDownloadURLFromPage(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "ip-to-cloudprovider/1.0 (https://github.com/BenjiTrapp/ip-to-cloudprovider)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching confirmation page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("confirmation page returned HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parsing confirmation page HTML: %w", err)
	}

	// Find the FIRST matching ServiceTags download link (most current)
	var downloadURL string
	doc.Find("a[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "download.microsoft.com") && strings.Contains(href, "ServiceTags") {
			downloadURL = href
			return false // stop at first match
		}
		return true
	})

	if downloadURL == "" {
		return "", fmt.Errorf("no ServiceTags download link found on page")
	}

	return downloadURL, nil
}

// fetchAndParseMicrosoftServiceTags downloads and parses a Microsoft ServiceTags JSON file.
func fetchAndParseMicrosoftServiceTags(url string) (*IPRange, error) {
	body, err := Fetch(url)
	if err != nil {
		return nil, fmt.Errorf("downloading service tags: %w", err)
	}
	return fetchAndParseMicrosoftServiceTagsFromBytes(body)
}

// fetchAndParseMicrosoftServiceTagsFromBytes parses Microsoft ServiceTags JSON from raw bytes.
func fetchAndParseMicrosoftServiceTagsFromBytes(data []byte) (*IPRange, error) {
	var tags serviceTagsFile
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, fmt.Errorf("parsing service tags JSON: %w", err)
	}

	ipRange := &IPRange{}
	seen := make(map[string]bool)

	for _, value := range tags.Values {
		for _, prefix := range value.Properties.AddressPrefixes {
			prefix = strings.TrimSpace(prefix)
			if prefix == "" || seen[prefix] {
				continue
			}
			seen[prefix] = true

			if ClassifyCIDR(prefix) {
				ipRange.IPv6 = append(ipRange.IPv6, prefix)
			} else {
				ipRange.IPv4 = append(ipRange.IPv4, prefix)
			}
		}
	}

	return ipRange, nil
}
