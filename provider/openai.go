package provider

import "fmt"

func init() {
	Register(Provider{
		Name:   "openai",
		URL:    "", // OpenAI publishes several per-bot JSON files that are merged
		Update: updateOpenAI,
	})
}

// openaiBotURLs are the JSON endpoints OpenAI publishes for its crawlers.
// OpenAI retired the legacy gptbot-ranges.txt file (now HTTP 403); ranges are
// served as per-bot JSON in the same "prefixes" format Google uses.
var openaiBotURLs = []string{
	"https://openai.com/gptbot.json",       // GPTBot (training crawler)
	"https://openai.com/searchbot.json",    // OAI-SearchBot
	"https://openai.com/chatgpt-user.json", // ChatGPT-User (user-triggered fetches)
}

// updateOpenAI fetches OpenAI's crawler IP ranges from all published bot JSON
// files and merges them, deduplicating overlapping prefixes.
func updateOpenAI(dataDir string) error {
	ipRange, err := fetchAndMergeOpenAI(openaiBotURLs)
	if err != nil {
		return err
	}
	return Save("openai", ipRange, dataDir)
}

// fetchAndMergeOpenAI downloads each URL, parses it as a prefix JSON file, and
// merges the results. Individual fetch/parse failures are best-effort and
// skipped; an error is returned only if none of the URLs yield data.
func fetchAndMergeOpenAI(urls []string) (*IPRange, error) {
	merged := &IPRange{}
	seen := make(map[string]bool)
	successCount := 0

	for _, url := range urls {
		body, err := Fetch(url)
		if err != nil {
			// Best-effort: a single missing bot file shouldn't fail the update
			continue
		}
		ranges, err := ParsePrefixJSON(body)
		if err != nil {
			continue
		}

		for _, cidr := range ranges.IPv4 {
			if !seen[cidr] {
				seen[cidr] = true
				merged.IPv4 = append(merged.IPv4, cidr)
			}
		}
		for _, cidr := range ranges.IPv6 {
			if !seen[cidr] {
				seen[cidr] = true
				merged.IPv6 = append(merged.IPv6, cidr)
			}
		}
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("all OpenAI bot range fetches failed")
	}

	return merged, nil
}
