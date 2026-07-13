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
| **Reputation check** | Flag malicious IPs via DNSBLs (Spamhaus & co.) and optional AbuseIPDB |
| **Shodan lookup** | Enrich IPs and domains with open ports, services, and CVEs |
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

The binary ships with an embedded snapshot of every provider's IP ranges, so it
works immediately — no download step required.

### Scan

```bash
ip-to-cloudprovider scan 8.8.8.8
```

### (Optional) Refresh IP ranges

The embedded data is a snapshot from build time. To fetch the latest ranges:

```bash
ip-to-cloudprovider -a
```

Fresh data is stored under a per-user data directory (e.g.
`~/.local/share/ip-to-cloudprovider` on Linux, `~/Library/...` on macOS) and
takes precedence over the embedded snapshot. Override it with `--data-dir` or
the `IP2CP_DATA_DIR` environment variable.

---

## Installation

### Using `go install`

```bash
go install github.com/BenjiTrapp/ip-to-cloudprovider@latest
ip-to-cloudprovider scan 8.8.8.8   # works immediately (embedded data)
ip-to-cloudprovider -a             # optional: refresh to the latest ranges
```

> **Note:** `go install` places the binary in `$(go env GOPATH)/bin` (usually
> `~/go/bin`). Make sure that directory is on your `PATH`:
> ```bash
> export PATH="$HOME/go/bin:$PATH"
> ```

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

# Also check each IP's reputation (malicious or not)
ip-to-cloudprovider scan 1.2.3.4 --reputation
ip-to-cloudprovider scan -f ips.txt -r -q -j
```

### Reputation / threat-intel check

The `--reputation` (`-r`) flag additionally checks each IP against
threat-intelligence sources and prints an overall verdict
(`CLEAN` / `SUSPICIOUS` / `MALICIOUS` / `UNKNOWN`):

```bash
$ ip-to-cloudprovider scan 1.2.3.4 -q -r
1.2.3.4              is not in the range of any provider  [CLEAN]
```

**No setup required.** Out of the box it queries keyless **DNS blocklists**
(no API key, no registration):

| Source | Zone |
|:-------|:-----|
| Spamhaus ZEN | `zen.spamhaus.org` |
| SpamCop | `bl.spamcop.net` |
| Barracuda | `b.barracudacentral.org` |
| UCEPROTECT L1 | `dnsbl-1.uceprotect.net` |

> **Note:** Public DNS resolvers (e.g. `8.8.8.8`, `1.1.1.1`) are blocked by
> Spamhaus and return an error code rather than results. The tool detects this
> and reports the source as errored instead of falsely flagging the IP. For
> reliable DNSBL lookups, use a local or ISP resolver.

**Optional API sources.** [AbuseIPDB](https://www.abuseipdb.com) can be enabled
with a free API key, supplied via config or the `ABUSEIPDB_API_KEY` environment
variable.

**Configuration.** Which sources are active is controlled by a YAML config file.
A missing file falls back to the keyless defaults above. Default location:

- Linux/macOS: `~/.config/ip-to-cloudprovider/reputation.yaml`
- Override: `--reputation-config <path>` or `IP2CP_REPUTATION_CONFIG=<path>`

See [`reputation.example.yaml`](reputation.example.yaml) for a template:

```yaml
dnsbls:
  - name: spamhaus-zen
    zone: zen.spamhaus.org
    score: 80
    # enabled: false   # turn a source off
abuseipdb:
  enabled: true
  api_key: ""          # or set ABUSEIPDB_API_KEY
  max_age_days: 90
```

> The reputation and Shodan settings live in the **same** config file — see the
> [Shodan lookup](#shodan-lookup) section for its `shodan:` block.

### Shodan lookup

The `shodan` command enriches IPs **and domains** with the intelligence
[Shodan](https://www.shodan.io) holds about them — open ports, detected
services, tags, and known CVEs. Domains are resolved to an IP via Shodan first.

```bash
# Single IP
ip-to-cloudprovider shodan 8.8.8.8

# Domain (resolved via Shodan) and IP together
ip-to-cloudprovider shodan example.com 1.1.1.1

# From a file, JSON output
ip-to-cloudprovider shodan -f targets.txt -q -j

# Piped from stdin
cat targets.txt | ip-to-cloudprovider shodan -q
```

Example output:

```text
=== 8.8.8.8 ===
  Location:    Mountain View, United States
  Org:         Google LLC
  Ports:       53, 443
  Services:
    - 53/udp Google DNS
    - 443/tcp
  Tags:        cloud
  Updated:     2026-07-10T12:00:00
```

**API key required.** The `shodan` command needs a Shodan API key, supplied via
the same config file (under the `shodan:` key) or the `SHODAN_API_KEY`
environment variable:

```yaml
shodan:
  enabled: true
  api_key: ""          # or set SHODAN_API_KEY
```

Point at a specific config with `--shodan-config <path>` (defaults to the same
per-user config file as the reputation settings).

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
| `--data-dir` | | Directory for IP range data files (default: per-user data dir; falls back to embedded snapshot) |
| `--version` | | Print version information |

`scan`-specific flags:

| Flag | Short | Description |
|:-----|:------|:------------|
| `--reputation` | `-r` | Also check each IP against threat-intel sources (DNSBLs, AbuseIPDB) |
| `--reputation-config` | | Path to reputation config file (default: per-user config dir) |
| `--stats` | | Show summary statistics after scan |
| `--file` | `-f` | Read IPs from file (one per line) |

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
├── reputation/
│   ├── reputation.go       Checker, Source interface, verdict aggregation
│   ├── dnsbl.go            DNS blocklist source (Spamhaus, SpamCop, ...)
│   ├── abuseipdb.go        AbuseIPDB API source (optional, needs key)
│   └── config.go           YAML config: which sources are active
├── shodan/
│   ├── shodan.go           Shodan REST client (host lookup + DNS resolve)
│   └── config.go           YAML config: Shodan API key
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
