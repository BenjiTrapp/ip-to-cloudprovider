package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/fatih/color"

	"strings"

	"github.com/BenjiTrapp/ip-to-cloudprovider/microsoft"
	"github.com/spf13/cobra"
)

const banner = `
   ____   ______     _______                _____               _    __       
  /  _/__/_  __/__  / ___/ /  ___  __ _____/ / _ \_______ _  __(_)__/ /__ ____
 _/ // _ \/ / / _ \/ /__/ /__/ _ \/ // / _  / ___/ __/ _ \ |/ / / _  / -_) __/
/___/ .__/_/  \___/\___/____/\___/\_,_/\_,_/_/  /_/  \___/___/_/\_,_/\__/_/   
   /_/                                                                        
`

type IPRange struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

var debug bool
var updateAll bool

var providers = []struct {
	name   string
	url    string
	parser func(data []byte) *IPRange
}{
	{"amazon", "https://ip-ranges.amazonaws.com/ip-ranges.json", parseAmazon},
	{"cloudflare", "https://api.cloudflare.com/client/v4/ips", parseCloudflare},
	{"github", "https://api.github.com/meta", parseGitHub},
	{"google", "https://www.gstatic.com/ipranges/goog.txt", parseGoogle},
	{"openai", "https://openai.com/gptbot-ranges.txt", parseOpenAI},
	{"microsoft", "NONE", nil},
}

func main() {
	fmt.Print(banner)

	fmt.Println("-------------------------------------------------------")

	var rootCmd = &cobra.Command{
		Use:   "ipranges",
		Short: "Manage IP ranges for various providers",
		Run: func(cmd *cobra.Command, args []string) {
			if updateAll {
				for _, provider := range providers {
					if provider.name != "microsoft" {
						updateIPRanges(provider.name, provider.url)
					}
				}
				microsoft.Download()
			} else {
				cmd.Help()
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&updateAll, "update-all", "a", false, "Update IP ranges for all providers")

	var checkIPCmd = &cobra.Command{
		Use:   "check-ip [ip]",
		Short: "Check if an IP belongs to any provider's range",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ip := args[0]
			checkIP(ip)
		},
	}

	checkIPCmd.Flags().BoolVarP(new(bool), "update", "c", false, "Check if an IP belongs to any provider's range")

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
		providerCmd.Flags().BoolVarP(&updateAll, "update-all", "a", false, "Update IP ranges for all providers")
		rootCmd.AddCommand(providerCmd)
	}

	var checkFileCmd = &cobra.Command{
		Use:   "check-file",
		Short: "Check if IPs from a file belong to any provider's range",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := args[0]
			checkIPsFromFile(filePath)
		},
	}

	checkFileCmd.Flags().StringP("file", "f", "", "Path to the file containing IP addresses")

	rootCmd.AddCommand(checkFileCmd)

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

	body, err := io.ReadAll(resp.Body)
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
	fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProviderName(capitalizeFirst(providerName)))
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

func colorizeProviderName(providerName string) string {
	var c *color.Color

	switch providerName {
	case "Microsoft":
		c = color.New(color.FgBlue).Add(color.Bold)
	case "Github":
		c = color.New(color.FgBlack).Add(color.BgWhite).Add(color.Bold)
	case "Amazon":
		c = color.New(color.FgYellow).Add(color.Bold)
	case "Cloudflare":
		c = color.New(color.FgHiRed).Add(color.BgYellow).Add(color.Bold)
	case "Google":
		c = color.New(color.FgRed).Add(color.Bold)
	case "Openai":
		c = color.New(color.FgCyan).Add(color.Bold)
	default:
		c = color.New(color.FgWhite)
	}

	return c.Sprint(providerName)
}

func checkIP(ip string) {
	for _, provider := range providers {
		ipRanges := loadIPRanges(provider.name)
		if ipRanges != nil && (isIPInRange(ip, ipRanges.IPv4) || isIPInRange(ip, ipRanges.IPv6)) {
			fmt.Printf("%-20s is in the range of %s\n", ip, colorizeProviderName(capitalizeFirst(provider.name)))
			return
		}
	}
	fmt.Printf("%-15s is not in the range of any provider\n", ip)
}

func checkIPsFromFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := scanner.Text()
		checkIP(ip)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %s\n", err)
	}
}

func isIPInRange(ip string, ranges []string) bool {
	parsedIP := net.ParseIP(ip)
	for _, cidr := range ranges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			if debug {
				fmt.Printf("Error parsing CIDR %s: %s\n", cidr, err)
			}
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

	err = os.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Printf("Error writing data to %s: %s\n", fileName, err)
	}
}

func loadIPRanges(providerName string) *IPRange {
	fileName := fmt.Sprintf("%s/ipranges.json", providerName)
	data, err := os.ReadFile(fileName)
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
	case "cloudflare":
		parser = parseCloudflare
	case "github":
		parser = parseGitHub
	case "google":
		parser = parseGoogle
	case "openai":
		parser = parseOpenAI
	case "microsoft":
		return nil
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

func parseCloudflare(data []byte) *IPRange {
	var result map[string]interface{}
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil
	}

	resultMap, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil
	}

	ipRange := &IPRange{
		IPv4: toStringArray(resultMap["ipv4_cidrs"]),
		IPv6: toStringArray(resultMap["ipv6_cidrs"]),
	}

	return ipRange
}

func toStringArray(data interface{}) []string {
	var result []string
	if data != nil {
		if dataSlice, ok := data.([]interface{}); ok {
			for _, item := range dataSlice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
		}
	}
	return result
}
