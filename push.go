package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const pushHistoryFileName = ".wssh_push_history"


// RunPushInstall handles SCP transfer, remote extraction, and logging
func RunPushInstall(payloadAlias, hostAlias string, cfg *Config) error {
	// 1. Look up the payload in the config
	localFilePath, exists := cfg.Payloads[payloadAlias]
	if !exists {
		return fmt.Errorf("payload alias '%s' not found in ~/.wssh.yaml under 'payloads'", payloadAlias)
	}

	// Expand ~/ to absolute path if necessary
	if strings.HasPrefix(localFilePath, "~/") {
		homeDir, _ := os.UserHomeDir()
		localFilePath = filepath.Join(homeDir, localFilePath[2:])
	}

	if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
		return fmt.Errorf("payload file does not exist: %s", localFilePath)
	}

	// 2. Gather global SSH args
	sshArgs := []string{}
	ignoreKeys := true
	if cfg.Settings.IgnoreKeyChanges != nil {
		ignoreKeys = *cfg.Settings.IgnoreKeyChanges
	}
	if ignoreKeys {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}

	// 3. Generate a clean remote filename (e.g., dotfiles.tgz)
	remoteFileName := fmt.Sprintf("%s.tgz", payloadAlias)
	fmt.Printf("📦 Uploading %s to %s as %s...\n", localFilePath, hostAlias, remoteFileName)

	// 4. Execute SCP
	scpArgs := append(sshArgs, localFilePath, fmt.Sprintf("%s:%s", hostAlias, remoteFileName))
	scpCmd := exec.Command("scp", scpArgs...)
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("SCP failed: %v", err)
	}

	// 5. Execute SSH to untar (Notice we don't delete the file after extraction) 
	fmt.Printf("⚙️  Extracting %s on %s (archive will remain on host)...\n", remoteFileName, hostAlias)
	remoteCmd := fmt.Sprintf("tar -xzf %s -C ~/", remoteFileName)
	
	sshRunArgs := append(sshArgs, hostAlias, remoteCmd)
	sshCmd := exec.Command("ssh", sshRunArgs...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("remote extraction failed: %v", err)
	}

	// 6. Log it to history
	err := LogPushConnection(hostAlias, payloadAlias)
	if err != nil {
		fmt.Printf("Warning: Failed to log push history: %v\n", err)
	}

	fmt.Println("✅ Push install complete!")
	return nil
}

// LogPushConnection appends the push event to the tracking file
func LogPushConnection(alias, filename string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	historyPath := filepath.Join(homeDir, pushHistoryFileName)

	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("%s,%s,%s\n", timestamp, alias, filename)

	_, err = f.WriteString(entry)
	return err
}
