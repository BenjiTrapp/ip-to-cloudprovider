package reputation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// abuseIPDBURL is the AbuseIPDB v2 check endpoint.
const abuseIPDBURL = "https://api.abuseipdb.com/api/v2/check"

// AbuseIPDB is an API-based source backed by https://www.abuseipdb.com.
// It requires a free API key, supplied via config or the ABUSEIPDB_API_KEY
// environment variable. The response's abuseConfidenceScore (0-100) is used
// directly as the source score.
type AbuseIPDB struct {
	apiKey    string
	maxAgeDay int // consider reports from the last N days
	client    *http.Client
}

// NewAbuseIPDB constructs an AbuseIPDB source. maxAgeDays defaults to 90 when
// non-positive.
func NewAbuseIPDB(apiKey string, maxAgeDays int) *AbuseIPDB {
	if maxAgeDays <= 0 {
		maxAgeDays = 90
	}
	return &AbuseIPDB{
		apiKey:    apiKey,
		maxAgeDay: maxAgeDays,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implements Source.
func (a *AbuseIPDB) Name() string { return "abuseipdb" }

// abuseIPDBResponse mirrors the fields we consume from the /check endpoint.
type abuseIPDBResponse struct {
	Data struct {
		AbuseConfidenceScore int    `json:"abuseConfidenceScore"`
		TotalReports         int    `json:"totalReports"`
		CountryCode          string `json:"countryCode"`
		Domain               string `json:"domain"`
		IsWhitelisted        bool   `json:"isWhitelisted"`
		UsageType            string `json:"usageType"`
	} `json:"data"`
	Errors []struct {
		Detail string `json:"detail"`
	} `json:"errors"`
}

// Check implements Source, querying the AbuseIPDB /check endpoint.
func (a *AbuseIPDB) Check(ctx context.Context, ip string) SourceResult {
	res := SourceResult{Source: a.Name()}

	if a.apiKey == "" {
		res.Err = "no API key configured"
		return res
	}

	q := url.Values{}
	q.Set("ipAddress", ip)
	q.Set("maxAgeInDays", fmt.Sprintf("%d", a.maxAgeDay))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, abuseIPDBURL+"?"+q.Encode(), nil)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	req.Header.Set("Key", a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		res.Err = err.Error()
		return res
	}

	var parsed abuseIPDBResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		res.Err = fmt.Sprintf("decoding response: %v", err)
		return res
	}

	if len(parsed.Errors) > 0 {
		res.Err = parsed.Errors[0].Detail
		return res
	}
	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return res
	}

	res.Score = parsed.Data.AbuseConfidenceScore
	res.Listed = parsed.Data.AbuseConfidenceScore > 0
	if parsed.Data.UsageType != "" {
		res.Categories = append(res.Categories, parsed.Data.UsageType)
	}
	res.Detail = fmt.Sprintf("confidence %d%%, %d reports",
		parsed.Data.AbuseConfidenceScore, parsed.Data.TotalReports)
	if parsed.Data.IsWhitelisted {
		res.Detail += ", whitelisted"
	}
	return res
}
