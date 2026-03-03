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

// GetKeyForHost dynamically determines the SSH Key path by matching the host alias
func GetKeyForHost(alias string, cfg *Config) string {
	for envName, envData := range cfg.Settings.SSHAgentEnvs {
		if envName == "default" {
			continue
		}
		if strings.HasPrefix(alias, envName) {
			return expandPath(envData.Key)
		}
	}
	if defaultEnv, exists := cfg.Settings.SSHAgentEnvs["default"]; exists {
		return expandPath(defaultEnv.Key)
	}
	return ""
}

// WarnKeyExpiration checks if the key for a specific host is expiring/expired and warns
func WarnKeyExpiration(alias string, cfg *Config) {
	keyPath := GetKeyForHost(alias, cfg)
	if keyPath == "" {
		return // No key configured for this host, ignore
	}

	fileInfo, err := os.Stat(keyPath)
	if err != nil {
		fmt.Printf("⚠️  \033[1;33mWarning: Could not find key file: %s\033[0m\n", keyPath)
		time.Sleep(2 * time.Second)
		return
	}

	hours := cfg.Settings.AgentExpirationHours
	if hours == 0 {
		hours = 23.5
	}

	expirationTime := fileInfo.ModTime().Add(time.Duration(hours * float64(time.Hour)))
	timeRemaining := time.Until(expirationTime)

	if timeRemaining < 0 {
		// Expired
		fmt.Printf("⚠️  \033[1;31mWARNING: Your SSH key for %s expired %v ago!\033[0m\n", alias, -timeRemaining.Round(time.Minute))
		time.Sleep(2 * time.Second) // Pause so the user can read it before iTerm takes focus
	} else if timeRemaining < 60*time.Minute {
		// Expiring soon (< 60 mins)
		fmt.Printf("⚠️  \033[1;33mWARNING: Your SSH key for %s will expire in %v!\033[0m\n", alias, timeRemaining.Round(time.Minute))
		time.Sleep(2 * time.Second) // Pause so the user can read it
	}
}

// CheckAndPrimeAgents uses the dynamic YAML configuration to prime sockets
func CheckAndPrimeAgents(cfg *Config) error {
	if len(cfg.Settings.SSHAgentEnvs) == 0 {
		return fmt.Errorf("no ssh_agent_envs found in ~/.wssh.yaml under settings")
	}

	// Print validation message
	checkEnv := cfg.Settings.AuthCheckEnv
	if checkEnv == "" { 
		checkEnv = "default" 
	}
	targetKey := expandPath(cfg.Settings.SSHAgentEnvs[checkEnv].Key)
	fileInfo, _ := os.Stat(targetKey)
	fmt.Printf("\033[1;32m[VALID]\033[0m Keys are ready (updated %s).\n\n", fileInfo.ModTime().Format(time.Kitchen))

	// Loop through dynamic config to prime agents
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