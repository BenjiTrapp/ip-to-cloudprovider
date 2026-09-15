package provider

func init() {
	Register(Provider{
		Name:  "google",
		URL:   "https://www.gstatic.com/ipranges/goog.txt",
		Parse: parseGoogleTxt,
	})
	Register(Provider{
		Name:  "googlecloud",
		URL:   "https://www.gstatic.com/ipranges/cloud.json",
		Parse: parseGoogleJSON,
	})
	Register(Provider{
		Name:  "googlebot",
		URL:   "https://developers.google.com/search/apis/ipranges/googlebot.json",
		Parse: parseGoogleJSON,
	})
}

// parseGoogleTxt parses Google's plain-text IP range list (one CIDR per line).
func parseGoogleTxt(data []byte) (*IPRange, error) {
	return ParsePlainTextCIDRs(data)
}

// parseGoogleJSON parses Google's JSON IP range format (cloud.json, googlebot.json).
func parseGoogleJSON(data []byte) (*IPRange, error) {
	return ParsePrefixJSON(data)
}
