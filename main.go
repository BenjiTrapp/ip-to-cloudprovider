package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/BenjiTrapp/ip-to-cloudprovider/provider"
)

const banner = `
   ____   ______     _______                _____               _    __       
  /  _/__/_  __/__  / ___/ /  ___  __ _____/ / _ \_______ _  __(_)__/ /__ ____
 _/ // _ \/ / / _ \/ /__/ /__/ _ \/ // / _  / ___/ __/ _ \ |/ / / _  / -_) __/
/___/ .__/_/  \___/\___/____/\___/\_,_/\_,_/_/  /_/  \___/___/_/\_,_/\__/_/   
   /_/                                                                        
`

func main() {
	fmt.Print(banner)
	fmt.Println("-------------------------------------------------------")

	// Data directory: current working directory (preserves existing behavior)
	dataDir := "."

	rootCmd := &cobra.Command{
		Use:   "ip-to-cloudprovider",
		Short: "Manage and check IP ranges for cloud providers",
	}

	// --update-all / -a flag on root
	var updateAll bool
	rootCmd.Flags().BoolVarP(&updateAll, "update-all", "a", false, "Update IP ranges for all providers")
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if updateAll {
			updateAllProviders(dataDir)
		} else {
			cmd.Help()
		}
	}

	// check-ip command
	checkIPCmd := &cobra.Command{
		Use:   "check-ip [ip]",
		Short: "Check if an IP belongs to any provider's range",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkIP(args[0], dataDir)
		},
	}

	// check-file command
	checkFileCmd := &cobra.Command{
		Use:   "check-file [file]",
		Short: "Check if IPs from a file belong to any provider's range",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkIPsFromFile(args[0], dataDir)
		},
	}

	// Per-provider subcommands with --update flag
	for _, p := range provider.Registry {
		p := p // capture loop variable
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

	rootCmd.AddCommand(checkIPCmd)
	rootCmd.AddCommand(checkFileCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func updateAllProviders(dataDir string) {
	for _, p := range provider.Registry {
		p := p
		if err := provider.UpdateProvider(&p, dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", p.Name, err)
			continue
		}
		fmt.Printf("%-20s IP ranges updated successfully\n", colorizeProvider(p.Name))
	}
}

func checkIP(ip, dataDir string) {
	name := provider.CheckIP(ip, dataDir)
	if name != "" {
		fmt.Printf("%-20s is in the range of %s\n", ip, colorizeProvider(name))
	} else {
		fmt.Printf("%-15s is not in the range of any provider\n", ip)
	}
}

func checkIPsFromFile(filePath, dataDir string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip == "" {
			continue
		}
		checkIP(ip, dataDir)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}
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
