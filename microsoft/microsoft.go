package microsoft

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
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

	// Sort & uniq
	sortAndUnique("microsoft/microsoft-all.json", "microsoft/microsoft-ipv4.json")
	fmt.Println("microsoft IP Ranges updated successfully")
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

func sortAndUnique(inputFile, ipv4File string) {
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
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		fmt.Println("Error marshaling to JSON:", err)
		os.Exit(1)
	}

	err = os.WriteFile(ipv4File, jsonResult, 0644)
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
		result = append(result, trimmed)
	}
	return result
}
