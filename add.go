package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RunAddInteractive launches a CLI wizard to add a new host to wssh and ssh config
func RunAddInteractive(cfg *Config) error {
	scanner := bufio.NewScanner(os.Stdin)

	// Helper to prompt and capture input
	ask := func(prompt string) string {
		fmt.Print(prompt)
		scanner.Scan()
		return strings.TrimSpace(scanner.Text())
	}

	fmt.Println("--- Add New Host ---")

	alias := ask("Alias (e.g., dev-web-02): ")
	if alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}

	hostname := ask("FQDN or IP Address: ")
	if hostname == "" {
		return fmt.Errorf("hostname/IP cannot be empty")
	}

	tagsInput := ask("Tags (comma separated, optional): ")
	var tags []string
	if tagsInput != "" {
		for _, t := range strings.Split(tagsInput, ",") {
			tags = append(tags, strings.TrimSpace(t))
		}
	}

	// Display existing groups to help the user choose
	fmt.Println("\nExisting Groups:")
	for _, g := range cfg.Groups {
		fmt.Printf("  - %s\n", g.Name)
	}
	groupName := ask("Group (type an existing one or create a new one): ")
	if groupName == "" {
		return fmt.Errorf("group cannot be empty")
	}

	// --- AUTH TYPE SELECTION ---
	fmt.Println("\nAvailable Auth Types:")
	fmt.Println("  1) key")
	fmt.Println("  2) password")
	authInput := ask("Select Auth Type (1 or 2) [default: 1]: ")
	
	authType := "key" // Default
	if authInput == "2" || strings.ToLower(authInput) == "password" {
		authType = "password"
	}

	// --- SSH AGENT SELECTION ---
	var identityFile string
	if authType == "key" {
		fmt.Println("\nAvailable SSH Agent Environments:")
		
		// Extract map keys into a slice so we can number them predictably
		var envNames []string
		for envName := range cfg.Settings.SSHAgentEnvs {
			envNames = append(envNames, envName)
		}
		
		for i, name := range envNames {
			fmt.Printf("  %d) %s\n", i+1, name)
		}
		
		envChoice := ask(fmt.Sprintf("Select Env (1-%d or name): ", len(envNames)))
		
		// Check if they typed a number; if so, map it back to the string name
		selectedEnv := envChoice
		for i, name := range envNames {
			if envChoice == fmt.Sprintf("%d", i+1) {
				selectedEnv = name
				break
			}
		}

		if envConfig, exists := cfg.Settings.SSHAgentEnvs[selectedEnv]; exists {
			identityFile = envConfig.Key
		} else {
			fmt.Println("\nWarning: Invalid env choice. No IdentityFile will be added to SSH config.")
		}
	}
	
	// 1. Update wssh.yaml Data Structure
	groupFound := false
	for i, g := range cfg.Groups {
		if strings.EqualFold(g.Name, groupName) {
			cfg.Groups[i].Hosts = append(cfg.Groups[i].Hosts, Host{
				Alias:    alias,
				Hostname: hostname,
				Tags:     tags,
			})
			groupFound = true
			break
		}
	}
	
	// If it's a brand new group, create it
	if !groupFound {
		cfg.Groups = append(cfg.Groups, Group{
			Name:  groupName,
			Hosts: []Host{{Alias: alias, Hostname: hostname, Tags: tags}},
		})
	}

	// 2. Write to ~/.wssh.yaml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	wsshPath := filepath.Join(homeDir, ".wssh.yaml")
	
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}
	
	if err := os.WriteFile(wsshPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write ~/.wssh.yaml: %v", err)
	}
	fmt.Println("\n✅ Added successfully to ~/.wssh.yaml")

	// 3. Append to ~/.ssh/config
	sshConfigPath := filepath.Join(homeDir, ".ssh", "config")
	f, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open ~/.ssh/config: %v", err)
	}
	defer f.Close()

	sshBlock := fmt.Sprintf("\nHost %s\n    HostName %s\n", alias, hostname)
	if identityFile != "" {
		sshBlock += fmt.Sprintf("    IdentityFile %s\n", identityFile)
	}

	if _, err := f.WriteString(sshBlock); err != nil {
		return fmt.Errorf("failed to write to ~/.ssh/config: %v", err)
	}
	fmt.Println("✅ Appended successfully to ~/.ssh/config")

	return nil
}
