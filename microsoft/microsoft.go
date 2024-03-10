package microsoft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func Download() {
	ids := []string{"56519", "57063", "57064", "57062"}
	for _, id := range ids {
		err := DownloadAndParse(id)
		if err != nil {
			log.Fatalf("Error in ID %s: %v", id, err)
		}
	}

	SortAndUnique("microsoft/microsoft-all.json", "microsoft/ipranges.json")
	fmt.Println("Microsoft IP Ranges updated successfully")
}

func DownloadAndParse(id string) error {
	url := fmt.Sprintf("https://www.microsoft.com/en-us/download/confirmation.aspx?id=%s", id)

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("ERROR fetching URL: %w", err)
	}
	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatalf("Error parsing document: %v", err)
	}

	var downloadURL string
	document.Find("a").Each(func(index int, element *goquery.Selection) {
		href, exists := element.Attr("href")
		if exists && strings.Contains(href, "ServiceTags_") {
			downloadURL = href
			return
		}
	})

	if downloadURL == "" {
		return fmt.Errorf("ERROR - No download URL found")
	}

	DownloadAndSave(downloadURL, "microsoft/microsoft-all.json")

	return nil
}

func DownloadAndSave(url, fileName string) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error fetching URL: %v", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	err = ioutil.WriteFile(fileName, body, 0644)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}
}

// Rest of the code remains the same

func SortAndUnique(inputFile, ipRangesFile string) {
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
