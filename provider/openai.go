package provider

func init() {
	Register(Provider{
		Name:  "openai",
		URL:   "https://openai.com/gptbot-ranges.txt",
		Parse: parseOpenAI,
	})
}

// parseOpenAI parses OpenAI's plain-text CIDR list.
func parseOpenAI(data []byte) (*IPRange, error) {
	return ParsePlainTextCIDRs(data)
}
