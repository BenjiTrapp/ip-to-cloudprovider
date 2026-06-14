<p align="center">
  <img height="200" src="static/logo.png" alt="IP to CloudProvider">
</p>

<h1 align="center">IP to CloudProvider</h1>

<p align="center">
  <strong>Instantly identify which cloud provider owns any IP address.</strong><br>
  Fast, concurrent, and always up-to-date.
</p>

<p align="center">
  <a href="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml"><img src="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/daily-scraper.yml/badge.svg" alt="IP Ranges Update"></a>
  <a href="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml"><img src="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/ipscanner.yml/badge.svg" alt="IP Scanner"></a>
  <a href="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml"><img src="https://github.com/BenjiTrapp/ip-to-cloudprovider/actions/workflows/go.yml/badge.svg" alt="Quality Check"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://github.com/BenjiTrapp/ip-to-cloudprovider/blob/main/LICENSE"><img src="https://img.shields.io/github/license/BenjiTrapp/ip-to-cloudprovider?style=flat" alt="License"></a>
</p>

---

## Why?

During incident response, threat hunting, or infrastructure audits you often need to quickly determine whether an IP belongs to AWS, Azure, GCP, or another cloud. This tool does that lookup **locally and instantly** against pre-downloaded CIDR registries -- no external API calls at scan time.

---

## Features

| | |
|---|---|
| **Multi-provider** | Match IPs against 15 provider registries simultaneously |
| **Blazing fast** | CIDRs parsed once, matched in-memory with concurrent workers |
| **Flexible input** | CLI args, file (`-f`), or piped stdin |
| **JSON output** | Machine-readable with `-j` for scripting and pipelines |
| **Summary stats** | Aggregate breakdown with `--stats` |
| **Selective updates** | Refresh a single provider or all at once |
| **Auto-refresh** | GitHub Actions updates IP ranges daily at midnight UTC |

---

## Supported Providers

| Provider | Source |
|:---------|:-------|
| Alibaba Cloud | ASN data (AS45102) via ipverse |
| Amazon AWS | `ip-ranges.amazonaws.com` |
| Anthropic (Claude) | `docs.anthropic.com/en/api/ip-addresses` |
| Cloudflare | Cloudflare API v4 |
| DigitalOcean | GeoIP CSV feed |
| GitHub (web) | GitHub `/meta` API |
| GitHub Actions | GitHub `/meta` API |
| GitHub Hooks | GitHub `/meta` API |
| GitHub Pages | GitHub `/meta` API |
| Google | `gstatic.com/ipranges/goog.txt` |
| Google Cloud | `gstatic.com/ipranges/cloud.json` |
| Googlebot | Google Search APIs |
| Hetzner | ASN data (AS24940) via ipverse |
| Microsoft Azure | ServiceTags JSON (4 clouds, deduplicated) |
| OpenAI | `openai.com/gptbot-ranges.txt` |

---

## Quick Start

### Install

```bash
go install github.com/BenjiTrapp/ip-to-cloudprovider@latest
```

### Download IP ranges

```bash
ip-to-cloudprovider -a
```

### Scan

```bash
ip-to-cloudprovider scan 8.8.8.8
```

That's it. Three commands to go from zero to identifying cloud IPs.

---

## Installation

### Using `go install`

```bash
go install github.com/BenjiTrapp/ip-to-cloudprovider@latest
ip-to-cloudprovider -a   # download provider data
```

### From source

```bash
git clone https://github.com/BenjiTrapp/ip-to-cloudprovider.git
cd ip-to-cloudprovider
make build
```

---

## Usage

### Update IP ranges

```bash
# All providers at once
ip-to-cloudprovider -a

# Individual provider
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

# From a file (one IP per line)
ip-to-cloudprovider scan -f ips.txt

# Piped from stdin
cat ips.txt | ip-to-cloudprovider scan -q

# JSON output for scripting
ip-to-cloudprovider scan 8.8.8.8 -q -j

# With summary statistics
ip-to-cloudprovider scan -f ips.txt --stats
```

### List providers

```bash
# Text output showing data status
ip-to-cloudprovider list

# JSON output
ip-to-cloudprovider list -j
```

### Legacy command

```bash
ip-to-cloudprovider scan-file demo_ips.txt
```

### Global Flags

| Flag | Short | Description |
|:-----|:------|:------------|
| `--quiet` | `-q` | Suppress banner output |
| `--json` | `-j` | Output results as JSON |
| `--data-dir` | | Directory for IP range data files (default: `.`) |
| `--version` | | Print version information |

---

## Demo

<p align="center">
  <img src="static/demo.gif" alt="Demo" width="700">
</p>

---

## Architecture

```
.
├── main.go                 CLI entry point (Cobra commands & output formatting)
├── provider/
│   ├── provider.go         Core types, registry, Fetch, Save/Load, CIDR validation
│   ├── matcher.go          Pre-loaded batch IP matcher with concurrency
│   ├── alibaba.go          Alibaba Cloud (AS45102 BGP data)
│   ├── amazon.go           Amazon AWS
│   ├── anthropic.go        Anthropic/Claude docs scraper
│   ├── cloudflare.go       Cloudflare API
│   ├── digitalocean.go     DigitalOcean CSV parser
│   ├── github.go           GitHub /meta (4 sub-providers, dedup fetch)
│   ├── google.go           Google / Google Cloud / Googlebot
│   ├── hetzner.go          Hetzner Online (AS24940 BGP data)
│   ├── microsoft.go        Azure ServiceTags (HTML scrape + JSON parse)
│   └── openai.go           OpenAI plain-text CIDR
├── .github/workflows/
│   ├── daily-scraper.yml   Nightly IP range refresh
│   ├── ipscanner.yml       Auto-scan on ips_to_scan.txt change
│   └── go.yml              CI: vet, fmt, test, race detector
└── Makefile                Build, test, lint, demo targets
```

---

## GitHub Actions Workflows

| Workflow | Trigger | Purpose |
|:---------|:--------|:--------|
| **Daily Scraper** | Cron (midnight UTC) | Keeps IP ranges fresh automatically |
| **IP Scanner** | Push to `ips_to_scan.txt` | Scans IPs and posts results as a GitHub Issue |
| **Quality Check** | Push / PR | Runs `go vet`, `gofmt`, tests, and race detector |

---

## Development

```bash
make test       # Run tests
make lint       # Vet + format check
make build      # Build binary with version tag
make update     # Build + update all provider data
make demo       # Build + scan demo_ips.txt
```

```bash
# Run tests with race detector
go test -race ./...
```

---

## Adding a New Provider

Create a file in `provider/` with an `init()` function that calls `Register()`:

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

That's all it takes -- the registry auto-discovers providers at startup.

---

## Contributing

Contributions are welcome! Whether it's a new provider, a bug fix, or a performance improvement -- open a pull request and let's make it happen.

---

<p align="center">
  <sub>Built with Go. Kept fresh by GitHub Actions. Use responsibly.</sub>
</p>
