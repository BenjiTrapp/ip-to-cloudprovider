package reputation

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// DNSBL is a DNS-based blocklist source (e.g. Spamhaus ZEN). It works without
// any API key: the IP's octets are reversed, the zone suffix is appended, and
// a DNS A lookup is performed. A successful lookup (typically 127.0.0.x) means
// the IP is listed. The returned address encodes which sub-list matched.
type DNSBL struct {
	// name identifies the source (e.g. "spamhaus-zen").
	name string
	// zone is the DNSBL zone suffix (e.g. "zen.spamhaus.org").
	zone string
	// score is the confidence assigned when an IP is listed by this blocklist.
	score int
	// resolver is used for lookups; nil means the default net.Resolver.
	resolver *net.Resolver
}

// NewDNSBL constructs a DNSBL source.
func NewDNSBL(name, zone string, score int) *DNSBL {
	return &DNSBL{name: name, zone: zone, score: score, resolver: net.DefaultResolver}
}

// Name implements Source.
func (d *DNSBL) Name() string { return d.name }

// Check implements Source. Only IPv4 is supported by most DNSBL zones; IPv6 and
// unparseable input yield an un-listed result rather than an error.
func (d *DNSBL) Check(ctx context.Context, ip string) SourceResult {
	res := SourceResult{Source: d.name}

	query, ok := reverseIPv4(ip)
	if !ok {
		// Not an IPv4 address; DNSBLs used here are IPv4-only. Report as
		// not-listed without an error so it doesn't skew the verdict.
		return res
	}

	host := query + "." + d.zone
	addrs, err := d.resolver.LookupHost(ctx, host)
	if err != nil {
		// NXDOMAIN is the normal "not listed" answer, not a failure.
		var dnsErr *net.DNSError
		if errAsDNS(err, &dnsErr) && (dnsErr.IsNotFound) {
			return res
		}
		res.Err = err.Error()
		return res
	}

	// A DNSBL answers with one or more 127.0.0.x addresses that encode which
	// sub-list matched. Addresses in 127.255.255.0/24 are error codes (e.g.
	// Spamhaus returns 127.255.255.254 when queried via a public/open resolver),
	// NOT real listings — treating them as hits would flag every IP.
	var listed []string
	for _, a := range addrs {
		switch {
		case isDNSBLError(a):
			res.Err = fmt.Sprintf("blocklist returned error code %s (public resolver?)", a)
			return res
		case isDNSBLListing(a):
			listed = append(listed, a)
		}
	}

	if len(listed) == 0 {
		return res
	}

	res.Listed = true
	res.Score = d.score
	res.Detail = fmt.Sprintf("listed (%s)", strings.Join(listed, ", "))
	return res
}

// isDNSBLListing reports whether a returned address is a genuine listing code,
// i.e. in 127.0.0.0/24 (the conventional DNSBL response block).
func isDNSBLListing(addr string) bool {
	ip := net.ParseIP(addr).To4()
	return ip != nil && ip[0] == 127 && ip[1] == 0 && ip[2] == 0
}

// isDNSBLError reports whether a returned address is an error/status code,
// i.e. in 127.255.255.0/24 (used by Spamhaus and others to signal problems
// such as queries from public resolvers or rate limiting).
func isDNSBLError(addr string) bool {
	ip := net.ParseIP(addr).To4()
	return ip != nil && ip[0] == 127 && ip[1] == 255 && ip[2] == 255
}

// reverseIPv4 reverses the octets of an IPv4 address for DNSBL queries, e.g.
// "1.2.3.4" -> "4.3.2.1". Returns false for non-IPv4 input.
func reverseIPv4(ip string) (string, bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", false
	}
	v4 := parsed.To4()
	if v4 == nil {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d.%d", v4[3], v4[2], v4[1], v4[0]), true
}

// errAsDNS is a small helper wrapping errors.As for *net.DNSError.
func errAsDNS(err error, target **net.DNSError) bool {
	if e, ok := err.(*net.DNSError); ok {
		*target = e
		return true
	}
	return false
}
