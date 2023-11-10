package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	// Public cloud
	downloadAndParse("56519")
	// US Gov
	downloadAndParse("57063")
	// Germany
	downloadAndParse("57064")
	// China
	downloadAndParse("57062")

	// Sort & uniq
	sortAndUnique("microsoft-all.txt", "microsoft-ipv4.txt", "microsoft-ipv6.txt")
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

	downloadAndSave(downloadURL, "microsoft-all.txt")
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

func sortAndUnique(inputFile, ipv4File, ipv6File string) {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	sort.Strings(lines)

	var ipv4Lines, ipv6Lines []string
	seen := make(map[string]bool)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !seen[line] {
			seen[line] = true
			if strings.Contains(line, ":") {
				ipv6Lines = append(ipv6Lines, line)
			} else {
				ipv4Lines = append(ipv4Lines, line)
			}
		}
	}

	ipv4Content := strings.Join(ipv4Lines, "\n")
	err = os.WriteFile(ipv4File, []byte(ipv4Content), 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		os.Exit(1)
	}

	ipv6Content := strings.Join(ipv6Lines, "\n")
	err = os.WriteFile(ipv6File, []byte(ipv6Content), 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		os.Exit(1)
	}
}
