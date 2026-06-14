package provider

import "fmt"

const (
	hetznerIPv4URL = "https://raw.githubusercontent.com/ipverse/asn-ip/master/as/24940/ipv4-aggregated.txt"
	hetznerIPv6URL = "https://raw.githubusercontent.com/ipverse/asn-ip/master/as/24940/ipv6-aggregated.txt"
)

func init() {
	Register(Provider{
		Name:   "hetzner",
		URL:    hetznerIPv4URL,
		Update: updateHetzner,
	})
}

// updateHetzner fetches both IPv4 and IPv6 aggregated CIDR lists for
// Hetzner Online (AS24940) and merges them.
func updateHetzner(dataDir string) error {
	ipv4Data, err := Fetch(hetznerIPv4URL)
	if err != nil {
		return fmt.Errorf("fetching Hetzner IPv4 ranges: %w", err)
	}

	ipv6Data, err := Fetch(hetznerIPv6URL)
	if err != nil {
		return fmt.Errorf("fetching Hetzner IPv6 ranges: %w", err)
	}

	ipRange := &IPRange{
		IPv4: parseCommentedCIDRs(string(ipv4Data)),
		IPv6: parseCommentedCIDRs(string(ipv6Data)),
	}

	return Save("hetzner", ipRange, dataDir)
}
