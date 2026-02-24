package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Could not determine home directory: %v", err)
	}
	configPath := filepath.Join(homeDir, ".wssh.yaml")

	// Ensure config exists (using the function from our previous steps)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		generateDefaultConfig(configPath)
		fmt.Printf("Created a default configuration file at: %s\n", configPath)
		os.Exit(0)
	}

	// Capture the full 'cfg' object so we can access cfg.Macros
	cfg, searchableHosts, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading config from %s: %v", configPath, err)
	}

	var rootCmd = &cobra.Command{
		Use:   "wssh [host] [layout]",
		Short: "wssh is an iTerm2 SSH orchestrator",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				var completions []string
				for _, host := range searchableHosts {
					if strings.HasPrefix(host.Alias, toComplete) {
						completions = append(completions, fmt.Sprintf("%s\t%s", host.Alias, host.GroupName))
					}
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			}

			// Autocomplete for the second argument (layouts)
			if len(args) == 1 {
				layouts := []string{"single\tStandard pane", "2h\tTwo horizontal panes", "2v\tTwo vertical panes", "3h\tThree horizontal panes", "3v\tThree vertical panes", "4g\t2x2 Grid"}
				return layouts, cobra.ShellCompDirectiveNoFileComp
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Check keys before doing anything else
			if err := CheckKeyExpiration(cfg); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			// No args? Open the TUI Menu!
			if len(args) == 0 {
				selectedHost := RunTUI(searchableHosts)
				if selectedHost != nil {
					fmt.Printf("Connecting to %s...\n", selectedHost.Alias)
					err := LaunchLayout(*selectedHost, "single", cfg)
					if err != nil {
						log.Fatalf("Failed to launch session: %v", err)
					}
				}
				return
			}

			// Args provided? CLI Mode!
			hostAlias := args[0]
			layout := "single" // default
			if len(args) > 1 {
				layout = args[1]
			}

			// Find the full SearchableHost object from the alias
			var targetHost SearchableHost
			found := false
			for _, h := range searchableHosts {
				if h.Alias == hostAlias {
					targetHost = h
					found = true
					break
				}
			}

			// Fallback: If they typed an ad-hoc hostname not in config
			if !found {
				targetHost = SearchableHost{Alias: hostAlias}
			}

			fmt.Printf("CLI Mode: Launching %s with layout '%s'\n", targetHost.Alias, layout)
			err := LaunchLayout(targetHost, layout, cfg)
			if err != nil {
				log.Fatalf("Failed to launch layout: %v", err)
			}
		},
	}

	var macroCmd = &cobra.Command{
		Use:   "macro [name]",
		Short: "Inject a macro into the active iTerm session",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var completions []string
			// Add autocomplete for your macros!
			for name, content := range cfg.Macros {
				if strings.HasPrefix(name, toComplete) {
					completions = append(completions, fmt.Sprintf("%s\t%s", name, content))
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("Available macros:")
				for name, content := range cfg.Macros {
					fmt.Printf("  %s: %s\n", name, content)
				}
				return
			}

			macroName := args[0]
			macroContent, exists := cfg.Macros[macroName]
			if !exists {
				log.Fatalf("Macro '%s' not found in ~/.wssh.yaml", macroName)
			}

			// Send the text to iTerm!
			err := SendMacro(macroContent)
			if err != nil {
				log.Fatalf("Failed to send macro: %v", err)
			}
		},
	}

	var authCmd = &cobra.Command{
		Use:   "auth",
		Short: "Check key expiration and prime SSH agents",
		Run: func(cmd *cobra.Command, args []string) {
			err := CheckAndPrimeAgents(cfg)
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	var historyCmd = &cobra.Command{
		Use:   "history",
		Short: "View recently connected hosts",
		Run: func(cmd *cobra.Command, args []string) {
			// Print the last 20 connections
			err := PrintHistory(20)
			if err != nil {
				fmt.Printf("Error reading history: %v\n", err)
			}
		},
	}
	var addCmd = &cobra.Command{
		Use:   "add",
		Short: "Interactively add a new host to config and SSH config",
		Run: func(cmd *cobra.Command, args []string) {
			err := RunAddInteractive(cfg)
			if err != nil {
				fmt.Printf("\nError adding host: %v\n", err)
			}
		},
	}

    // The direct CLI command
	var pushCmd = &cobra.Command{
		Use:   "pushinstall [payload-alias] [host-alias]",
		Short: "Push a configured .tgz payload to a host and extract it",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunPushInstall(args[0], args[1], cfg)
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
			}
		},
	}

	// The interactive menu command (used by the TUI)
	var pushMenuCmd = &cobra.Command{
		Use:    "pushmenu [host-alias]",
		Hidden: true, // Hides it from the --help output to keep things clean
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			hostAlias := args[0]
			
			if len(cfg.Payloads) == 0 {
				fmt.Println("❌ No payloads configured in ~/.wssh.yaml.")
				fmt.Println("Press Enter to return...")
				bufio.NewReader(os.Stdin).ReadBytes('\n')
				return
			}

			fmt.Printf("--- Push Payload to %s ---\n", hostAlias)
			
			var aliases []string
			for a := range cfg.Payloads {
				aliases = append(aliases, a)
			}
			
			for i, a := range aliases {
				fmt.Printf("  %d) %s (%s)\n", i+1, a, cfg.Payloads[a])
			}

			fmt.Print("\nSelect a payload (number or name): ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			choice := strings.TrimSpace(scanner.Text())

			selectedAlias := choice
			for i, a := range aliases {
				if choice == fmt.Sprintf("%d", i+1) {
					selectedAlias = a
					break
				}
			}

			fmt.Println()
			err := RunPushInstall(selectedAlias, hostAlias, cfg)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			
			fmt.Print("\nPress Enter to return to menu...")
			scanner.Scan()
		},
	}
	var captureCmd = &cobra.Command{
		Use:   "capture [jb-alias] node [node-list] cap [filter...]",
		Short: "Dynamically resolve nodes on a jumpbox and stream PCAPs to Wireshark",
		Args:  cobra.MinimumNArgs(4), // Needs at least: alias, "node", "x", "cap"
		Run: func(cmd *cobra.Command, args []string) {
			jbAlias := args[0]
			
			// Validate syntax
			if args[1] != "node" || args[3] != "cap" {
				fmt.Println("❌ Invalid syntax.")
				fmt.Println("Usage: wssh capture <jb-alias> node <3,4> cap <port 443>")
				return
			}

			nodeList := args[2]

			// Everything after "cap" are the tcpdump parameters
			filter := ""
			if len(args) > 4 {
				// We removed the single quotes! Now it passes flags cleanly.
				filter = strings.Join(args[4:], " ")
			}

			// 1. Resolve the inner node FQDNs AND get the socket path
			targetFQDNs, sockPath, err := ResolveTargetNodes(jbAlias, nodeList, cfg)
			if err != nil {
				fmt.Printf("\nError resolving nodes: %v\n", err)
				return
			}

			// 2. Launch the parallel capture!
			err = RunCapture(targetFQDNs, filter, sockPath, cfg)
			if err != nil {
				fmt.Printf("\nError during capture: %v\n", err)
			}
		},
	}
	var runCmd = &cobra.Command{
		Use:   "run [script.sh] [host-alias]",
		Short: "Stream and execute a local script on a remote host without leaving a footprint",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			scriptPath := args[0]
			hostAlias := args[1]
			
			err := RunScript(scriptPath, hostAlias, cfg)
			if err != nil {
				fmt.Printf("\nError: %v\n", err)
			}
		},
	}
	var listCmd = &cobra.Command{
		Use:   "list [search-terms...]",
		Short: "List all hosts or search using multiple terms (AND logic)",
		Run: func(cmd *cobra.Command, args []string) {
			// 1. Convert all arguments to lowercase search terms
			var terms []string
			for _, arg := range args {
				terms = append(terms, strings.ToLower(arg))
			}

			fmt.Println("--- Host Aliases ---")
			count := 0
			
			for _, host := range searchableHosts {
				aliasLower := strings.ToLower(host.Alias)
				
				// 2. Assume it's a match, then try to disprove it (Logical AND)
				match := true
				for _, term := range terms {
					if !strings.Contains(aliasLower, term) {
						match = false
						break // Failed the AND check, move to the next host
					}
				}

				if match {
					fmt.Printf("  %-30s [Group: %s]\n", host.Alias, host.GroupName)
					count++
				}
			}

			if count == 0 {
				fmt.Printf("❌ No hosts found matching: %s\n", strings.Join(args, " "))
			} else {
				fmt.Printf("\nTotal matches: %d\n", count)
			}
		},
	}

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(pushMenuCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(macroCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Keep your existing generateDefaultConfig function down here...
func generateDefaultConfig(path string) {
	// (Your existing implementation)
}
