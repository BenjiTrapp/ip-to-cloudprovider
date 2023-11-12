package microsoft

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// TestSortAndUnique tests the SortAndUnique function.
func TestSortAndUnique(t *testing.T) {
	// Test case with unsorted and duplicate data
	runTest(t, "testdata/unsorted_duplicate.txt", "testdata/ipranges.json", map[string][]string{
		"ipv4": {"192.168.1.1", "192.168.1.2", "192.168.1.3"},
		"ipv6": {},
	})

	// Test case with sorted and unique data
	runTest(t, "testdata/sorted_unique.txt", "testdata/ipranges.json", map[string][]string{
		"ipv4": {"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		"ipv6": {},
	})

	// Test case with mixed valid IPv4 and invalid data
	runTest(t, "testdata/mixed_data.txt", "testdata/ipranges.json", map[string][]string{
		"ipv4": {"192.168.1.1", "192.168.1.2", "192.168.1.3", "10.0.0.1", "10.0.0.2", "10.0.0.3"},
		"ipv6": {},
	})
}

// runTest is a helper function to run a single test case.
func runTest(t *testing.T, inputFile, ipRangesFile string, expectedOutput map[string][]string) {
	// Run the SortAndUnique function
	SortAndUnique(inputFile, ipRangesFile)

	// Read the result from the ipRangesFile
	content, err := os.ReadFile(ipRangesFile)
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}

	// Unmarshal the JSON content into a map
	var result map[string][]string
	err = json.Unmarshal(content, &result)
	if err != nil {
		t.Fatalf("Error unmarshaling JSON: %v", err)
	}

	// Sort the expected and actual results before comparing
	sort.Strings(expectedOutput["ipv4"])
	sort.Strings(result["ipv4"])

	// Compare the result with the expected output
	if !reflect.DeepEqual(result, expectedOutput) {
		t.Errorf("Result does not match expected output.\nExpected: %v\nActual: %v", expectedOutput, result)
	}
}
