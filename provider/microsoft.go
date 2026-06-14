package provider

import (
	"encoding/json"
	"fmt"
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
var microsoftDownloadIDs = map[string]string{
	"Public":  "56519",
	"USGov":   "57063",
	"China":   "57064",
	"Germany": "57062",
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
func updateMicrosoft(dataDir string) error {
	ipRange := &IPRange{}
	seen := make(map[string]bool)

	for cloud, id := range microsoftDownloadIDs {
		downloadURL, err := discoverMicrosoftDownloadURL(id)
		if err != nil {
			return fmt.Errorf("discovering download URL for Azure %s (id=%s): %w", cloud, id, err)
		}

		ranges, err := fetchAndParseMicrosoftServiceTags(downloadURL)
		if err != nil {
			return fmt.Errorf("fetching Azure %s service tags: %w", cloud, err)
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
	}

	return Save("microsoft", ipRange, dataDir)
}

// discoverMicrosoftDownloadURL scrapes the Microsoft download confirmation page
// to find the actual JSON download link.
func discoverMicrosoftDownloadURL(id string) (string, error) {
	url := fmt.Sprintf("https://www.microsoft.com/en-us/download/confirmation.aspx?id=%s", id)
	return discoverMicrosoftDownloadURLFromPage(url)
}

// discoverMicrosoftDownloadURLFromPage fetches the given page URL and extracts
// the ServiceTags download link from the HTML.
func discoverMicrosoftDownloadURLFromPage(pageURL string) (string, error) {
	resp, err := httpClient.Get(pageURL)
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

	var downloadURL string
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "download.microsoft.com") && strings.Contains(href, "ServiceTags") {
			downloadURL = href
		}
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
