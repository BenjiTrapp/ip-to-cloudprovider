[![IP Ranges Update](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml)
[![ipscanner](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml)
[![Quality Check after Commit](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml/badge.svg)](https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml)

<p align="center">
<img height="200" src="static/logo.png">
<br> IP To CloudProvider
</p>

A fast command-line tool for identifying which cloud provider owns a given IP address. Supports batch lookups, JSON output, piped input, and automatic daily updates via GitHub Actions.

## Features

- **Multi-provider lookup** - Check IPs against 12 provider registries simultaneously
- **Batch scanning** - Process thousands of IPs with concurrent matching
- **Multiple input methods** - CLI args, file input (`-f`), or piped stdin
- **JSON output** - Machine-readable output with `--json` for scripting and pipelines
- **Summary statistics** - Aggregate results with `--stats`
- **Per-provider updates** - Update individual providers or all at once
- **Automatic daily updates** - GitHub Actions workflow keeps IP ranges fresh
- **Pre-loaded matcher** - Parses CIDRs once, then matches in-memory for speed

## Supported Providers

| Provider | Source |
|----------|--------|
| Amazon AWS | `ip-ranges.amazonaws.com` |
| Cloudflare | Cloudflare API v4 |
| DigitalOcean | GeoIP CSV feed |
| GitHub (web) | GitHub `/meta` API |
| GitHub Actions | GitHub `/meta` API |
| GitHub Hooks | GitHub `/meta` API |
| GitHub Pages | GitHub `/meta` API |
| Google | `gstatic.com/ipranges/goog.txt` |
| Google Cloud | `gstatic.com/ipranges/cloud.json` |
| Googlebot | Google Search APIs |
| Microsoft Azure | ServiceTags JSON (4 clouds, deduplicated) |
| OpenAI | `openai.com/gptbot-ranges.txt` |

## Installation

### Using `go install`

```bash
go install github.com/BenjiTrapp/ip-to-cloudprovider@latest
```

Then download the IP range data:
```bash
ip-to-cloudprovider -a
```

### From source

```bash
git clone https://github.com/BenjiTrapp/ip-to-cloudprovider.git
cd ip-to-cloudprovider
make build
```

## Usage

### Update IP ranges

```bash
# Update all providers
ip-to-cloudprovider -a

# Update a single provider
ip-to-cloudprovider amazon --update
ip-to-cloudprovider microsoft --update
```

### Scan IPs

```bash
# Single IP
ip-to-cloudprovider scan 8.8.8.8

# Multiple IPs
ip-to-cloudprovider scan 13.224.1.1 198.41.200.1 64.225.84.5

# Short alias
ip-to-cloudprovider s 8.8.8.8

# From a file
ip-to-cloudprovider scan -f ips.txt

# Piped from stdin
cat ips.txt | ip-to-cloudprovider scan -q

# JSON output for scripting
ip-to-cloudprovider scan 8.8.8.8 -q -j

# With summary statistics
ip-to-cloudprovider scan -f ips.txt --stats
```

### Scan file (legacy command)

```bash
ip-to-cloudprovider scan-file demo_ips.txt
```

### List providers

```bash
# Text output showing data status
ip-to-cloudprovider list

# JSON output
ip-to-cloudprovider list -j
```

### Global flags

| Flag | Short | Description |
|------|-------|-------------|
| `--quiet` | `-q` | Suppress banner output |
| `--json` | `-j` | Output results as JSON |
| `--data-dir` | | Directory for IP range data files (default: `.`) |
| `--version` | | Print version information |

## Demo

![](/static/demo.gif)

## GitHub Action Workflows

- **Daily Scraper** - Updates IP ranges at midnight UTC every day
- **IP Scanner** - Triggered when `ips_to_scan.txt` is modified; posts results as a GitHub Issue
- **Quality Checks** - Runs `go vet`, `gofmt`, tests, and race detector on every push/PR

## Architecture

```
provider/
  provider.go      - Core types, registry, Fetch, Save/Load, CIDR validation
  matcher.go       - Pre-loaded batch IP matcher with concurrency threshold
  amazon.go        - Amazon AWS parser
  cloudflare.go    - Cloudflare API parser
  digitalocean.go  - DigitalOcean CSV parser
  github.go        - GitHub /meta (4 sub-providers) + dedup fetch
  google.go        - Google/GoogleCloud/Googlebot parsers
  microsoft.go     - Azure ServiceTags HTML scraping + JSON parsing
  openai.go        - OpenAI plain-text CIDR parser
main.go            - CLI entry point (cobra)
```

## Development

```bash
# Run tests
make test

# Run tests with race detector
go test -race ./...

# Lint
make lint

# Build with version tag
make build
```

## Contribution

Contributions are welcome! If you'd like to add support for a new provider or improve the existing code, please submit a pull request.

To add a new provider, create a file in `provider/` with an `init()` function that calls `Register()`:

```go
package provider

func init() {
    Register(Provider{
        Name:  "myprovider",
        URL:   "https://example.com/ranges.json",
        Parse: parseMyProvider,
    })
}

func parseMyProvider(data []byte) (*IPRange, error) {
    // Parse the response into IPv4 and IPv6 CIDR lists
    return &IPRange{IPv4: ipv4s, IPv6: ipv6s}, nil
}
```

**Note:** This tool is provided as-is, without any warranties. Use it responsibly and respect the terms of service of the supported providers.
