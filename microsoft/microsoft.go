package microsoft

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func Download() {
	// Public cloud
	downloadAndParse("56519")
	// US Gov
	downloadAndParse("57063")
	// Germany
	downloadAndParse("57064")
	// China
	downloadAndParse("57062")

	sortAndUnique("microsoft/microsoft-all.json", "microsoft/ipranges.json")
	fmt.Println("Microsoft IP Ranges updated successfully")
}

func downloadAndParse(id string) {
	url := fmt.Sprintf("https://www.microsoft.com/en-us/download/confirmation.aspx?id=%s", id)

	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching URL:", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		fmt.Println("Error parsing document:", err)
		os.Exit(1)
	}

	var downloadURL string
	document.Find("a").Each(func(index int, element *goquery.Selection) {
		href, exists := element.Attr("href")
		if exists && strings.Contains(href, "ServiceTags_") {
			downloadURL = href
			return
		}
	})

	downloadAndSave(downloadURL, "microsoft/microsoft-all.json")
}

func downloadAndSave(url, fileName string) {
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching URL:", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	file, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Error creating file:", err)
		os.Exit(1)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		os.Exit(1)
	}
}

func sortAndUnique(inputFile, ipRangesFile string) {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	sort.Strings(lines)

	var ipv4Lines []string
	seen := make(map[string]bool)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !seen[line] {
			seen[line] = true
			ipv4Lines = append(ipv4Lines, line)
		}
	}

	result := map[string][]string{
		"ipv4": stripQuotes(ipv4Lines),
		"ipv6": []string{}, // Add "ipv6" key with an empty collection
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		fmt.Println("Error marshaling to JSON:", err)
		os.Exit(1)
	}

	err = os.WriteFile(ipRangesFile, jsonResult, 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		os.Exit(1)
	}
}

func stripQuotes(lines []string) []string {
	var result []string
	for _, line := range lines {
		trimmed := strings.Trim(line, "\"")
		trimmed = strings.TrimSuffix(trimmed, "\",")
		trimmed = strings.TrimSpace(trimmed)

		// Check if the trimmed line is a valid IPv4 address
		if isValidIPv4(trimmed) {
			result = append(result, trimmed)
		} else {
			// Check if the line contains IPv4 addresses as part of a larger string
			ipAddresses := extractIPv4Addresses(trimmed)
			result = append(result, ipAddresses...)
		}
	}
	return result
}

func extractIPv4Addresses(s string) []string {
	var ipAddresses []string
	re := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}\b`)
	matches := re.FindAllString(s, -1)
	ipAddresses = append(ipAddresses, matches...)
	return ipAddresses
}

func isValidIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}
