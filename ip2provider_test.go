package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
)

func TestUpdateIPRanges(t *testing.T) {
	// Mock HTTP server for testing
	mockData := []byte(`{"prefixes":[{"ip_prefix":"192.168.1.0/24"}],"ipv6_prefixes":[{"ipv6_prefix":"2001:db8::/32"}]}`)
	mockServer := mockHTTPServer(mockData)
	defer mockServer.Close()

	// Update IP ranges for the "amazon" provider
	updateIPRanges("amazon", "http://"+mockServer.Addr)

	// Check if the ipranges.json file is created for the "amazon" provider
	ipRange := loadIPRanges("amazon")
	if ipRange == nil {
		t.Fatal("Failed to load IP ranges")
	}

	// Check if the IP ranges are correctly parsed and saved
	expectedIPv4 := []string{"192.168.1.0/24"}
	expectedIPv6 := []string{"2001:db8::/32"}

	if !stringSlicesEqual(ipRange.IPv4, expectedIPv4) {
		t.Fatalf("IPv4 ranges mismatch. Expected: %v, Got: %v", expectedIPv4, ipRange.IPv4)
	}

	if !stringSlicesEqual(ipRange.IPv6, expectedIPv6) {
		t.Fatalf("IPv6 ranges mismatch. Expected: %v, Got: %v", expectedIPv6, ipRange.IPv6)
	}
}

func TestCheckIP(t *testing.T) {
	// Mock IP ranges for the "amazon" provider
	ipRange := &IPRange{
		IPv4: []string{"192.168.1.0/24"},
		IPv6: []string{"2001:db8::/32"},
	}

	// Save mock IP ranges to a file for the "amazon" provider
	saveIPRanges("amazon", ipRange)

	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	// Check if an IP is in the range for the "amazon" provider
	ip := "192.168.1.1"
	checkIP(ip)
	w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	expectedOutput := fmt.Sprintf("%s is in the range of amazon\n", ip)
	actualOutput := buf.String()

	if actualOutput != expectedOutput {
		t.Fatalf("Unexpected output. Expected: %s, Got: %s", expectedOutput, actualOutput)
	}
}

func stringSlicesEqual(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i, v := range slice1 {
		if v != slice2[i] {
			return false
		}
	}

	return true
}

func mockHTTPServer(responseData []byte) *http.Server {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(responseData)
	})

	server := &http.Server{
		Handler: mockHandler,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0") // Use a dynamic port
	if err != nil {
		fmt.Printf("Failed to start mock server: %v\n", err)
		return nil
	}
	server.Addr = listener.Addr().String()

	go func() {
		if err := server.Serve(listener); err != nil {
			fmt.Printf("Mock server error: %v\n", err)
		}
	}()

	return server
}
