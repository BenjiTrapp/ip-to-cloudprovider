package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateIPRanges(t *testing.T) {
	mockServer := createMockServer()
	defer mockServer.Close()

	// Test each provider's IP range update
	for _, provider := range providers {
		t.Run("Update IP Ranges - "+provider.name, func(t *testing.T) {
			updateIPRanges(provider.name, mockServer.URL)

			// Check if the ipranges.json file is created or updated.
			filePath := fmt.Sprintf("%s/ipranges.json", provider.name)
			_, err := os.Stat(filePath)
			assert.NoError(t, err, "Expected %s to be created, but it does not exist", filePath)
		})
	}
}

func TestCheckIP(t *testing.T) {
	mockServer := createMockServer()
	defer mockServer.Close()

	// Update IP ranges before running the check
	for _, provider := range providers {
		updateIPRanges(provider.name, mockServer.URL)
	}

	testCases := []struct {
		ip       string
		expected string
	}{
		{"203.0.113.0", "203.0.113.0     is not in the range of any provider\n"},
		{"198.41.128.0", "198.41.128.0         is in the range of Cloudflare\n"},
		{"192.30.255.0", "192.30.255.0         is in the range of Github\n"},
		{"13.224.15.0", "13.224.15.0          is in the range of Amazon\n"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Check_IP_-_ %s", tc.ip), func(t *testing.T) {
			output := captureOutput(func() {
				checkIP(tc.ip)
			})
			expected := normalizeOutput(tc.expected)
			output = normalizeOutput(output)
			assert.Equal(t, expected, output)
		})
	}
}

func TestCheckIPsFromFile(t *testing.T) {
	mockServer := createMockServer()
	defer mockServer.Close()

	// Update IP ranges before running the check
	for _, provider := range providers {
		updateIPRanges(provider.name, mockServer.URL)
	}

	// Create a temporary file with test IPs
	filePath := "test_ips.txt"
	testIPs := []string{"203.0.113.0", "8.8.8.8", "192.30.255.0", "13.224.15.0", "198.41.128.0", "13.67.177.0"}
	createTestIPFile(filePath, testIPs)
	defer os.Remove(filePath)

	testCases := []struct {
		filePath string
		expected string
	}{
		{"test_ips.txt", "203.0.113.0     is not in the range of any provider\n8.8.8.8         is not in the range of any provider\n192.30.255.0         is in the range of Github\n13.224.15.0          is in the range of Amazon\n198.41.128.0         is in the range of Cloudflare\n13.67.177.0     is not in the range of any provider"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Check_IPs_from_File_-_ %s", tc.filePath), func(t *testing.T) {
			output := captureOutput(func() {
				checkIPsFromFile(tc.filePath)
			})
			expected := normalizeOutput(tc.expected)
			output = normalizeOutput(output)
			assert.Equal(t, expected, output)
		})
	}
}

func createMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ip-ranges.amazonaws.com/ip-ranges.json":
			fmt.Fprint(w, `{"prefixes": [{"ip_prefix": "203.0.113.0"}], "ipv6_prefixes": []}`)
		case "/api.cloudflare.com/client/v4/ips":
			fmt.Fprint(w, `{"result": {"ipv4_cidrs": ["198.41.128.0"], "ipv6_cidrs": []}}`)
		case "/api.github.com/meta":
			fmt.Fprint(w, `{"web": ["192.30.255.0"]}`)
		case "/www.gstatic.com/ipranges/goog.txt":
			fmt.Fprint(w, `8.8.8.8
                          2001:4860:4860::8888`)
		case "/openai.com/gptbot-ranges.txt":
			fmt.Fprint(w, `13.224.15.0`)
		case "/some-microsoft-api-url":
			//fmt.Fprint(w, `{"ipv4": ["Microsoft_IPv4"], "ipv6": ["Microsoft_IPv6"]}`)
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
}

func captureOutput(f func()) string {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old // restoring the real stdout
	return string(out)
}

func createTestIPFile(filePath string, ips []string) {
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	for _, ip := range ips {
		file.WriteString(ip + "\n")
	}
}

func normalizeOutput(output string) string {
	return trimLeadingTrailingSpaces(output)
}

func trimLeadingTrailingSpaces(s string) string {
	return strings.TrimSpace(s)
}
