package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type IPRange struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

var updateAll bool

var providers = []struct {
	name   string
	url    string
	parser func(data []byte) *IPRange
}{
	{"amazon", "https://ip-ranges.amazonaws.com/ip-ranges.json", parseAmazon},
	{"github", "https://api.github.com/meta", parseGitHub},
	{"google", "https://www.gstatic.com/ipranges/goog.txt", parseGoogle},
	{"openai", "https://openai.com/gptbot-ranges.txt", parseOpenAI},
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ipranges",
		Short: "Manage IP ranges for various providers",
		Run: func(cmd *cobra.Command, args []string) {
			if updateAll {
				for _, provider := range providers {
					updateIPRanges(provider.name, provider.url)
				}
			} else {
				cmd.Help()
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&updateAll, "update-all", "", false, "Update IP ranges for all providers")

	var checkIPCmd = &cobra.Command{
		Use:   "check-ip [ip]",
		Short: "Check if an IP belongs to any provider's range",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ip := args[0]
			checkIP(ip)
		},
	}
	rootCmd.AddCommand(checkIPCmd)

	for _, provider := range providers {
		provider := provider
		providerCmd := &cobra.Command{
			Use:   provider.name,
			Short: fmt.Sprintf("Manage %s IP ranges", provider.name),
			Run: func(cmd *cobra.Command, args []string) {
				update, _ := cmd.Flags().GetBool("update")
				check, _ := cmd.Flags().GetBool("check")
				if update {
					updateIPRanges(provider.name, provider.url)
				} else if check {
					ip := args[0]
					checkIP(ip)
				} else {
					cmd.Help()
				}
			},
		}
		providerCmd.Flags().BoolVarP(&updateAll, "update-all", "", false, "Update IP ranges for all providers")
		rootCmd.AddCommand(providerCmd)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func updateIPRanges(providerName, url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching data for %s: %s\n", providerName, err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading data for %s: %s\n", providerName, err)
		return
	}

	ipRange := parseProviderData(providerName, body)
	if ipRange == nil {
		fmt.Printf("Error parsing data for %s\n", providerName)
		return
	}

	saveIPRanges(providerName, ipRange)
	fmt.Printf("%s IP ranges updated successfully\n", providerName)
}

func checkIP(ip string) {
	for _, provider := range providers {
		ipRanges := loadIPRanges(provider.name)
		if ipRanges != nil && (isIPInRange(ip, ipRanges.IPv4) || isIPInRange(ip, ipRanges.IPv6)) {
			fmt.Printf("%s is in the range of %s\n", ip, provider.name)
			return
		}
	}
	fmt.Printf("%s is not in the range of any provider\n", ip)
}

func isIPInRange(ip string, ranges []string) bool {
	parsedIP := net.ParseIP(ip)
	for _, cidr := range ranges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			fmt.Printf("Error parsing CIDR %s: %s\n", cidr, err)
			continue
		}
		if ipNet.Contains(parsedIP) {
			return true
		}
	}
	return false
}

func saveIPRanges(providerName string, ipRange *IPRange) {
	fileName := fmt.Sprintf("%s/ipranges.json", providerName)
	data, err := json.Marshal(ipRange)
	if err != nil {
		fmt.Printf("Error marshalling data for %s: %s\n", providerName, err)
		return
	}

	err = ioutil.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Printf("Error writing data to %s: %s\n", fileName, err)
	}
}

func loadIPRanges(providerName string) *IPRange {
	fileName := fmt.Sprintf("%s/ipranges.json", providerName)
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Printf("Error reading data from %s: %s\n", fileName, err)
		return nil
	}

	var ipRange IPRange
	err = json.Unmarshal(data, &ipRange)
	if err != nil {
		fmt.Printf("Error unmarshalling data for %s: %s\n", providerName, err)
		return nil
	}

	return &ipRange
}

func parseProviderData(providerName string, data []byte) *IPRange {
	var parser func(data []byte) *IPRange
	switch providerName {
	case "amazon":
		parser = parseAmazon
	case "github":
		parser = parseGitHub
	case "google":
		parser = parseGoogle
	case "openai":
		parser = parseOpenAI
	default:
		fmt.Printf("Unknown provider: %s\n", providerName)
		return nil
	}
	return parser(data)
}

func parseAmazon(data []byte) *IPRange {
	var result struct {
		Prefixes []struct {
			IP string `json:"ip_prefix"`
		} `json:"prefixes"`
		IPv6Prefixes []struct {
			IP string `json:"ipv6_prefix"`
		} `json:"ipv6_prefixes"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Printf("Error unmarshalling Amazon data: %s\n", err)
		return nil
	}

	ipRange := &IPRange{}
	for _, prefix := range result.Prefixes {
		ipRange.IPv4 = append(ipRange.IPv4, prefix.IP)
	}
	for _, prefix := range result.IPv6Prefixes {
		ipRange.IPv6 = append(ipRange.IPv6, prefix.IP)
	}

	return ipRange
}

func parseGitHub(data []byte) *IPRange {
	var result struct {
		Web []string `json:"web"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Printf("Error unmarshalling GitHub data: %s\n", err)
		return nil
	}

	ipRange := &IPRange{IPv4: result.Web}
	return ipRange
}

func parseGoogle(data []byte) *IPRange {
	lines := strings.Split(string(data), "\n")

	ipRange := &IPRange{}
	for _, line := range lines {
		if strings.Contains(line, "include:") {
			continue
		}

		ip := strings.Fields(line)[0]
		if strings.Contains(ip, ":") {
			ipRange.IPv6 = append(ipRange.IPv6, ip)
		} else {
			ipRange.IPv4 = append(ipRange.IPv4, ip)
		}
	}

	return ipRange
}

func parseOpenAI(data []byte) *IPRange {
	lines := strings.Split(string(data), "\n")

	ipRange := &IPRange{}
	for _, line := range lines {
		if strings.Contains(line, ":") {
			continue
		}

		ipRange.IPv4 = append(ipRange.IPv4, line)
	}

	return ipRange
}
