package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/BenjiTrapp/ip-to-cloudprovider/provider"
	"github.com/BenjiTrapp/ip-to-cloudprovider/reputation"
	"github.com/BenjiTrapp/ip-to-cloudprovider/shodan"
)

// version is set at build time via ldflags.
var version = "dev"

const banner = `
   ____   ______     _______                _____               _    __       
  /  _/__/_  __/__  / ___/ /  ___  __ _____/ / _ \_______ _  __(_)__/ /__ ____
 _/ // _ \/ / / _ \/ /__/ /__/ _ \/ // / _  / ___/ __/ _ \ |/ / / _  / -_) __/
/___/ .__/_/  \___/\___/____/\___/\_,_/\_,_/_/  /_/  \___/___/_/\_,_/\__/_/   
   /_/                                                                        
`

// Global flags
var (
	quiet            bool
	jsonOutput       bool
	showStats        bool
	dataDir          string
	checkRep         bool
	repConfigPath    string
	shodanConfigPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "ip-to-cloudprovider",
		Short:   "Manage and check IP ranges for cloud providers",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !quiet {
				fmt.Print(banner)
				fmt.Println("-------------------------------------------------------")
			}
		},
	}

	// Global persistent flags
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress banner output")
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output results as JSON")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", provider.DefaultDataDir(), "Directory for IP range data files")

	// --update-all / -a flag on root
	var updateAll bool
	rootCmd.Flags().BoolVarP(&updateAll, "update-all", "a", false, "Update IP ranges for all providers")
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if updateAll {
			updateAllProviders()
		} else {
			cmd.Help()
		}
	}

	// scan command
	scanCmd := &cobra.Command{
		Use:     "scan [ip...]",
		Aliases: []string{"check-ip", "s"},
		Short:   "Check if one or more IPs belong to any provider's range",
		Long: `Check if one or more IPs belong to any provider's range.

Accepts IPs as arguments, from stdin (pipe), or both.

Examples:
  ip-to-cloudprovider scan 8.8.8.8
  ip-to-cloudprovider s 8.8.8.8 1.1.1.1 13.224.0.1
  ip-to-cloudprovider scan --stats -f ips.txt
  ip-to-cloudprovider scan 1.2.3.4 --reputation
  echo "8.8.8.8" | ip-to-cloudprovider scan -q -j
  cat ips.txt | ip-to-cloudprovider scan -q -j`,
		Run: func(cmd *cobra.Command, args []string) {
			file, _ := cmd.Flags().GetString("file")
			ips := collectIPs(args, file)
			if len(ips) == 0 {
				fmt.Fprintln(os.Stderr, "Error: no IPs provided. Pass IPs as arguments, use -f <file>, or pipe via stdin.")
				os.Exit(1)
			}
			scanIPs(ips)
		},
	}
	scanCmd.Flags().StringP("file", "f", "", "Read IPs from file (one per line)")
	scanCmd.Flags().BoolVar(&showStats, "stats", false, "Show summary statistics after scan")
	scanCmd.Flags().BoolVarP(&checkRep, "reputation", "r", false, "Also check each IP against threat-intel sources (DNSBLs, AbuseIPDB)")
	scanCmd.Flags().StringVar(&repConfigPath, "reputation-config", "", "Path to reputation config file (default: per-user config dir)")

	// scan-file command (kept for backward compat)
	scanFileCmd := &cobra.Command{
		Use:     "scan-file [file]",
		Aliases: []string{"check-file", "sf"},
		Short:   "Check IPs from a file against all provider ranges",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ips := readIPsFromFile(args[0])
			scanIPs(ips)
		},
	}
	scanFileCmd.Flags().BoolVar(&showStats, "stats", false, "Show summary statistics after scan")

	// list command
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all supported providers and their data status",
		Run: func(cmd *cobra.Command, args []string) {
			listProviders()
		},
	}

	// shodan command
	shodanCmd := &cobra.Command{
		Use:     "shodan [ip-or-domain...]",
		Aliases: []string{"sh"},
		Short:   "Look up IPs or domains on Shodan (open ports, services, CVEs)",
		Long: `Look up one or more IPs or domains on Shodan.

Domains are resolved to an IP via Shodan before the host lookup. Requires a
Shodan API key, set in the config file (shodan.api_key) or the SHODAN_API_KEY
environment variable.

Accepts targets as arguments, from stdin (pipe), or from a file (-f).

Examples:
  ip-to-cloudprovider shodan 8.8.8.8
  ip-to-cloudprovider shodan example.com 1.1.1.1
  ip-to-cloudprovider shodan -f targets.txt -q -j
  cat targets.txt | ip-to-cloudprovider shodan -q`,
		Run: func(cmd *cobra.Command, args []string) {
			file, _ := cmd.Flags().GetString("file")
			targets := collectIPs(args, file)
			if len(targets) == 0 {
				fmt.Fprintln(os.Stderr, "Error: no targets provided. Pass IPs/domains as arguments, use -f <file>, or pipe via stdin.")
				os.Exit(1)
			}
			shodanScan(targets)
		},
	}
	shodanCmd.Flags().StringP("file", "f", "", "Read targets from file (one per line)")
	shodanCmd.Flags().StringVar(&shodanConfigPath, "shodan-config", "", "Path to config file with the Shodan API key (default: per-user config dir)")

	// Per-provider subcommands with --update flag
	for _, p := range provider.Registry {
		p := p
		cmd := &cobra.Command{
			Use:   p.Name,
			Short: fmt.Sprintf("Manage %s IP ranges", p.Name),
			Run: func(cmd *cobra.Command, args []string) {
				update, _ := cmd.Flags().GetBool("update")
				if update {
					if err := provider.UpdateProvider(&p, dataDir); err != nil {
						fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", p.Name, err)
						os.Exit(1)
					}
					fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider(p.Name))
				} else {
					cmd.Help()
				}
			},
		}
		cmd.Flags().BoolP("update", "u", false, fmt.Sprintf("Update %s IP ranges", p.Name))
		rootCmd.AddCommand(cmd)
	}

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(scanFileCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(shodanCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func updateAllProviders() {
	// Deduplicate GitHub meta fetches
	githubUpdated := false

	for _, p := range provider.Registry {
		p := p

		// Optimize: fetch GitHub /meta once for all GitHub sub-providers
		if provider.IsGitHubProvider(p.Name) {
			if !githubUpdated {
				if err := provider.UpdateGitHubAll(dataDir); err != nil {
					fmt.Fprintf(os.Stderr, "Error updating GitHub providers: %v\n", err)
				} else {
					fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider("github"))
					fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider("githubactions"))
					fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider("githubhooks"))
					fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider("githubpages"))
				}
				githubUpdated = true
			}
			continue
		}

		if err := provider.UpdateProvider(&p, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", p.Name, err)
			continue
		}
		fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider(p.Name))
	}
}

func scanIPs(ips []string) {
	if !provider.HasAnyData(dataDir) {
		fmt.Fprintln(os.Stderr, "Warning: no provider data found. Run 'ip-to-cloudprovider -a' to download IP ranges first.")
	}

	matcher := provider.NewMatcher(dataDir)
	results := matcher.MatchAll(ips)

	var reports []reputation.Report
	if checkRep {
		reports = runReputation(ips)
	}

	if jsonOutput {
		if checkRep {
			outputJSONWithReputation(results, reports)
		} else {
			outputJSON(results)
		}
	} else {
		if checkRep {
			outputTextWithReputation(results, reports)
		} else {
			outputText(results)
		}
		if showStats && len(results) > 1 {
			outputStats(results)
		}
	}
}

// runReputation checks all IPs against the configured threat-intel sources.
// Returns nil (and warns) if no sources are active or the config fails to load.
func runReputation(ips []string) []reputation.Report {
	cfg, err := reputation.LoadConfig(repConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading reputation config: %v\n", err)
		return nil
	}

	checker := reputation.NewChecker(cfg)
	if !checker.Enabled() {
		fmt.Fprintln(os.Stderr, "Warning: reputation check requested but no sources are active. Check your reputation config.")
		return nil
	}

	return checker.CheckAll(context.Background(), ips)
}

// shodanScan looks up each target on Shodan and prints the results.
func shodanScan(targets []string) {
	cfg, err := shodan.LoadConfig(shodanConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading Shodan config: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey() == "" {
		fmt.Fprintln(os.Stderr, "Error: no Shodan API key found. Set 'shodan.api_key' in the config file or the SHODAN_API_KEY environment variable.")
		os.Exit(1)
	}

	client := shodan.NewClient(cfg.APIKey())
	ctx := context.Background()

	results := make([]shodan.Result, len(targets))
	for i, t := range targets {
		results[i] = client.Scan(ctx, t)
	}

	if jsonOutput {
		outputShodanJSON(results)
	} else {
		outputShodanText(results)
	}
}

func listProviders() {
	if jsonOutput {
		type providerInfo struct {
			Name    string `json:"name"`
			HasData bool   `json:"has_data"`
		}
		var infos []providerInfo
		for _, p := range provider.Registry {
			infos = append(infos, providerInfo{
				Name:    p.Name,
				HasData: provider.HasData(p.Name, dataDir),
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(infos)
		return
	}

	fmt.Printf("%-20s %s\n", "PROVIDER", "STATUS")
	fmt.Printf("%-20s %s\n", "--------", "------")
	for _, p := range provider.Registry {
		status := color.RedString("no data")
		if provider.HasData(p.Name, dataDir) {
			status = color.GreenString("ready")
		}
		fmt.Printf("%-20s %s\n", colorizeProvider(p.Name), status)
	}
	fmt.Printf("\n%d providers registered\n", len(provider.Registry))
}

// ---------------------------------------------------------------------------
// IP collection
// ---------------------------------------------------------------------------

// collectIPs gathers IPs from command args, an optional file, and stdin (if piped).
func collectIPs(args []string, file string) []string {
	var ips []string

	// From arguments
	for _, arg := range args {
		ip := strings.TrimSpace(arg)
		if ip != "" {
			ips = append(ips, ip)
		}
	}

	// From file flag
	if file != "" {
		ips = append(ips, readIPsFromFile(file)...)
	}

	// From stdin if piped
	if isStdinPiped() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			ip := strings.TrimSpace(scanner.Text())
			if ip != "" {
				ips = append(ips, ip)
			}
		}
	}

	return ips
}

// isStdinPiped returns true if stdin has piped data (not a terminal).
func isStdinPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// readIPsFromFile reads IPs from a file, one per line.
func readIPsFromFile(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var ips []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" {
			ips = append(ips, ip)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	return ips
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

func outputJSON(results []provider.MatchResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
	}
}

// combinedResult merges a provider match with its reputation report for JSON output.
type combinedResult struct {
	provider.MatchResult
	Reputation *reputation.Report `json:"reputation,omitempty"`
}

// outputJSONWithReputation emits the provider match and reputation report per IP.
func outputJSONWithReputation(results []provider.MatchResult, reports []reputation.Report) {
	combined := make([]combinedResult, len(results))
	for i, r := range results {
		combined[i] = combinedResult{MatchResult: r}
		if i < len(reports) {
			rep := reports[i]
			combined[i].Reputation = &rep
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(combined); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
	}
}

func outputShodanJSON(results []shodan.Result) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
	}
}

// outputShodanText prints a human-readable summary of each Shodan lookup.
func outputShodanText(results []shodan.Result) {
	for _, r := range results {
		header := colorizeTarget(r.Target)
		if r.IsDomain && r.ResolvedIP != "" {
			header = fmt.Sprintf("%s (%s)", colorizeTarget(r.Target), colorizeIP(r.ResolvedIP))
		}
		fmt.Printf("%s %s %s\n", headerColor.Sprint("==="), header, headerColor.Sprint("==="))

		if r.Err != "" {
			fmt.Printf("  %s\n\n", color.RedString("error: %s", r.Err))
			continue
		}
		if r.Host == nil {
			fmt.Printf("  %s\n\n", color.YellowString("no data"))
			continue
		}

		h := r.Host
		if loc := formatLocation(h); loc != "" {
			fmt.Printf("  %s %s\n", label("Location:"), loc)
		}
		if h.Org != "" {
			fmt.Printf("  %s %s\n", label("Org:"), h.Org)
		}
		if h.ISP != "" {
			fmt.Printf("  %s %s\n", label("ISP:"), h.ISP)
		}
		if h.OS != "" {
			fmt.Printf("  %s %s\n", label("OS:"), h.OS)
		}
		if len(h.Hostnames) > 0 {
			hosts := make([]string, len(h.Hostnames))
			for i, hn := range h.Hostnames {
				hosts[i] = colorizeURL(hn)
			}
			fmt.Printf("  %s %s\n", label("Hostnames:"), strings.Join(hosts, ", "))
		}
		if len(h.Ports) > 0 {
			fmt.Printf("  %s %s\n", label("Ports:"), colorizePorts(h.Ports, h.Services))
		}
		if len(h.Services) > 0 {
			fmt.Printf("  %s\n", label("Services:"))
			for _, s := range h.Services {
				fmt.Printf("    %s %s\n", color.New(color.Faint).Sprint("-"), colorizeService(s))
			}
		}
		if len(h.Tags) > 0 {
			tags := make([]string, len(h.Tags))
			for i, t := range h.Tags {
				tags[i] = tagColor.Sprint(t)
			}
			fmt.Printf("  %s %s\n", label("Tags:"), strings.Join(tags, " "))
		}
		if len(h.Vulns) > 0 {
			vulns := make([]string, len(h.Vulns))
			for i, v := range h.Vulns {
				vulns[i] = vulnColor.Sprintf(" %s ", v)
			}
			fmt.Printf("  %s %s\n", label("Vulns:"), strings.Join(vulns, " "))
		}
		if h.LastUpdate != "" {
			fmt.Printf("  %s %s\n", label("Updated:"), color.New(color.Faint).Sprint(h.LastUpdate))
		}
		fmt.Println()
	}
}

// formatLocation renders city/country from a Shodan host record.
func formatLocation(h *shodan.HostInfo) string {
	switch {
	case h.City != "" && h.Country != "":
		return h.City + ", " + h.Country
	case h.Country != "":
		return h.Country
	default:
		return ""
	}
}

// colorizePorts renders a sorted port list, coloring each port by the transport
// of its matching service (tcp vs udp) so protocols are visually distinct.
func colorizePorts(ports []int, services []shodan.Service) string {
	transportByPort := make(map[int]string, len(services))
	for _, s := range services {
		if _, ok := transportByPort[s.Port]; !ok {
			transportByPort[s.Port] = strings.ToLower(s.Transport)
		}
	}

	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = transportColor(transportByPort[p]).Sprintf("%d", p)
	}
	return strings.Join(parts, " ")
}

// colorizeService renders a single service as "port/transport product version"
// with the port+transport colored by protocol and the product highlighted.
func colorizeService(s shodan.Service) string {
	tc := transportColor(strings.ToLower(s.Transport))

	out := tc.Sprintf("%d", s.Port)
	if s.Transport != "" {
		out += tc.Sprint("/" + s.Transport)
	}
	if s.Product != "" {
		out += " " + color.New(color.FgHiWhite, color.Bold).Sprint(s.Product)
	}
	if s.Version != "" {
		out += " " + color.New(color.Faint).Sprint(s.Version)
	}
	return out
}

func outputText(results []provider.MatchResult) {
	for _, r := range results {
		ip := padColored(colorizeIP(r.IP), r.IP, 20)
		if r.Match {
			fmt.Printf("%s is in the range of %s\n", ip, colorizeProvider(r.Provider))
		} else {
			fmt.Printf("%s %s\n", ip, color.New(color.Faint).Sprint("is not in the range of any provider"))
		}
	}
}

// outputTextWithReputation prints the provider match plus a reputation verdict
// for each IP. reports is aligned by index with results.
func outputTextWithReputation(results []provider.MatchResult, reports []reputation.Report) {
	for i, r := range results {
		ip := padColored(colorizeIP(r.IP), r.IP, 20)
		if r.Match {
			fmt.Printf("%s is in the range of %s", ip, colorizeProvider(r.Provider))
		} else {
			fmt.Printf("%s %s", ip, color.New(color.Faint).Sprint("is not in the range of any provider"))
		}

		if i < len(reports) {
			rep := reports[i]
			fmt.Printf("  [%s]", colorizeVerdict(rep.Verdict, rep.Score))
			if hits := listedSources(rep); hits != "" {
				fmt.Printf(" %s", hits)
			}
		}
		fmt.Println()
	}
}

// listedSources returns a comma-separated list of sources that flagged the IP.
func listedSources(rep reputation.Report) string {
	var names []string
	for _, s := range rep.Sources {
		if s.Listed && s.Err == "" {
			names = append(names, s.Source)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "flagged by: " + strings.Join(names, ", ")
}

// colorizeVerdict renders a reputation verdict with an appropriate color.
func colorizeVerdict(v reputation.Verdict, score int) string {
	var c *color.Color
	switch v {
	case reputation.VerdictMalicious:
		c = color.New(color.FgHiWhite, color.BgRed, color.Bold)
	case reputation.VerdictSuspicious:
		c = color.New(color.FgBlack, color.BgYellow, color.Bold)
	case reputation.VerdictClean:
		c = color.New(color.FgGreen, color.Bold)
	default:
		c = color.New(color.FgWhite)
	}
	if v == reputation.VerdictMalicious || v == reputation.VerdictSuspicious {
		return c.Sprintf("%s %d%%", strings.ToUpper(string(v)), score)
	}
	return c.Sprint(strings.ToUpper(string(v)))
}

func outputStats(results []provider.MatchResult) {
	stats := provider.Summary(results)
	fmt.Println("\n--- Summary ---")
	fmt.Printf("Total: %d IPs scanned\n", len(results))

	// Sort provider names for consistent output
	var names []string
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if name == "unknown" {
			fmt.Printf("  %-18s %d\n", "No match:", stats[name])
		} else {
			fmt.Printf("  %-18s %d\n", colorizeProvider(name)+":", stats[name])
		}
	}
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// Shared colors for the various output elements, defined once so the palette
// stays consistent across commands.
var (
	headerColor = color.New(color.FgHiCyan, color.Bold)               // === section headers ===
	labelColor  = color.New(color.FgCyan, color.Bold)                 // "Ports:", "Org:", ...
	urlColor    = color.New(color.FgHiBlue, color.Underline)          // hostnames / URLs
	ipColor     = color.New(color.FgHiGreen)                          // IP addresses
	tagColor    = color.New(color.FgBlack, color.BgHiCyan)            // Shodan tags
	vulnColor   = color.New(color.FgHiWhite, color.BgRed, color.Bold) // CVEs
)

// label renders a right-padded, colored field label for aligned output.
func label(name string) string {
	return labelColor.Sprintf("%-11s", name)
}

// padColored left-aligns a colored string to a target column width. It pads
// based on the plain (uncolored) text length, since ANSI escape codes would
// otherwise be counted by fmt's width specifiers and misalign the columns.
func padColored(colored, plain string, width int) string {
	if pad := width - len(plain); pad > 0 {
		return colored + strings.Repeat(" ", pad)
	}
	return colored
}

// transportColor picks a color for a network transport: tcp and udp get
// distinct hues, anything else stays neutral.
func transportColor(transport string) *color.Color {
	switch transport {
	case "tcp":
		return color.New(color.FgHiGreen, color.Bold)
	case "udp":
		return color.New(color.FgHiMagenta, color.Bold)
	default:
		return color.New(color.FgHiWhite)
	}
}

// colorizeURL highlights a hostname or URL to make it stand out and clickable.
func colorizeURL(s string) string {
	return urlColor.Sprint(s)
}

// colorizeIP highlights an IP address.
func colorizeIP(s string) string {
	return ipColor.Sprint(s)
}

// colorizeTarget highlights a scan target: IPs are colored as IPs, everything
// else (domains) as a URL.
func colorizeTarget(s string) string {
	if net.ParseIP(s) != nil {
		return colorizeIP(s)
	}
	return colorizeURL(s)
}

func colorizeProvider(name string) string {
	var c *color.Color

	switch strings.ToLower(name) {
	case "microsoft":
		c = color.New(color.FgBlue, color.Bold)
	case "github", "githubactions", "githubhooks", "githubpages":
		c = color.New(color.FgBlack, color.BgWhite, color.Bold)
	case "amazon":
		c = color.New(color.FgYellow, color.Bold)
	case "cloudflare":
		c = color.New(color.FgHiRed, color.BgYellow, color.Bold)
	case "google", "googlecloud", "googlebot":
		c = color.New(color.FgRed, color.Bold)
	case "openai":
		c = color.New(color.FgCyan, color.Bold)
	case "digitalocean":
		c = color.New(color.FgBlue, color.Bold)
	case "alibaba":
		c = color.New(color.FgHiYellow, color.Bold)
	case "anthropic":
		c = color.New(color.FgHiMagenta, color.Bold)
	case "hetzner":
		c = color.New(color.FgRed, color.Bold)
	default:
		c = color.New(color.FgWhite)
	}

	return c.Sprint(capitalizeFirst(name))
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
