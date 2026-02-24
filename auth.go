package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// expandPath converts "~/" or "$HOME" into absolute paths for os.Stat
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return os.ExpandEnv(path)
}

// CheckKeyExpiration silently checks the configured key against the configured time limit
func CheckKeyExpiration(cfg *Config) error {
	// If the user hasn't configured agents in YAML, just skip the check
	if len(cfg.Settings.SSHAgentEnvs) == 0 {
		return nil
	}

	// Figure out which environment's key to check
	checkEnv := cfg.Settings.AuthCheckEnv
	if checkEnv == "" {
		checkEnv = "default" // Generic fallback instead of a company specific one
	}

	targetEnv, exists := cfg.Settings.SSHAgentEnvs[checkEnv]
	if !exists {
		return fmt.Errorf("auth_check_env '%s' not found in settings.ssh_agent_envs. Please update your ~/.wssh.yaml", checkEnv)
	}

	targetKey := expandPath(targetEnv.Key)
	fileInfo, err := os.Stat(targetKey)
	if err != nil {
		return fmt.Errorf("could not find key %s. Have you run the 2FA utility?", targetKey)
	}

	// Grab expiration hours, defaulting to 23.5 if they left it blank
	hours := cfg.Settings.AgentExpirationHours
	if hours == 0 {
		hours = 23.5
	}
	
	// Convert the float (23.5) into a time.Duration
	expirationLimit := time.Duration(hours * float64(time.Hour))

	if time.Since(fileInfo.ModTime()) > expirationLimit {
		return fmt.Errorf("\033[1;31m[EXPIRED]\033[0m Keys were last updated at %s.\nPlease run your manual 2FA utility, then run 'wssh auth'.", fileInfo.ModTime().Format(time.RFC822))
	}

	return nil
}

// CheckAndPrimeAgents uses the dynamic YAML configuration to prime sockets
func CheckAndPrimeAgents(cfg *Config) error {
	if len(cfg.Settings.SSHAgentEnvs) == 0 {
		return fmt.Errorf("no ssh_agent_envs found in ~/.wssh.yaml under settings")
	}

	// 1. Check expiration first
	if err := CheckKeyExpiration(cfg); err != nil {
		return err
	}

	// Print validation message
	checkEnv := cfg.Settings.AuthCheckEnv
	if checkEnv == "" { 
		checkEnv = "default" 
	}
	targetKey := expandPath(cfg.Settings.SSHAgentEnvs[checkEnv].Key)
	fileInfo, _ := os.Stat(targetKey)
	fmt.Printf("\033[1;32m[VALID]\033[0m Keys are fresh (updated %s).\n\n", fileInfo.ModTime().Format(time.Kitchen))

	// 2. Loop through dynamic config to prime agents
	for envName, config := range cfg.Settings.SSHAgentEnvs {
		fmt.Printf("--- Setting up Agent for: %s ---\n", envName)

		sockPath := expandPath(config.Sock)
		keyPath := expandPath(config.Key)

		checkCmd := exec.Command("ssh-add", "-l")
		checkCmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sockPath)
		
		if err := checkCmd.Run(); err == nil {
			fmt.Printf("Socket %s is alive.\n", sockPath)
		} else {
			fmt.Printf("Socket %s missing or dead. Starting new agent...\n", sockPath)
			os.Remove(sockPath)
			
			agentCmd := exec.Command("ssh-agent", "-a", sockPath)
			agentCmd.Run()
		}

		flushCmd := exec.Command("ssh-add", "-D")
		flushCmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sockPath)
		flushCmd.Run()

		if _, err := os.Stat(keyPath); err == nil {
			addCmd := exec.Command("ssh-add", keyPath)
			addCmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sockPath)
			
			if err := addCmd.Run(); err == nil {
				fmt.Printf("Successfully updated %s agent with %s\n\n", envName, keyPath)
			} else {
				fmt.Printf("Failed to add key to %s agent.\n\n", envName)
			}
		} else {
			fmt.Printf("Warning: Key file %s not found.\n\n", keyPath)
		}
	}
	return nil
}