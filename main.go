package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/BenjiTrapp/ip-to-cloudprovider/provider"
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
	quiet      bool
	jsonOutput bool
	showStats  bool
	dataDir    string
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
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", ".", "Directory for IP range data files")

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

	if jsonOutput {
		outputJSON(results)
	} else {
		outputText(results)
		if showStats && len(results) > 1 {
			outputStats(results)
		}
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

func outputText(results []provider.MatchResult) {
	for _, r := range results {
		if r.Match {
			fmt.Printf("%-20s is in the range of %s\n", r.IP, colorizeProvider(r.Provider))
		} else {
			fmt.Printf("%-20s is not in the range of any provider\n", r.IP)
		}
	}
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
