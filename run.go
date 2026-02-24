package main

import (
	"fmt"
	"os"
	"os/exec"
)

// RunScript streams a local script to a remote host and executes it in memory
func RunScript(scriptPath, hostAlias string, cfg *Config) error {
	// 1. Verify the local script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script file does not exist: %s", scriptPath)
	}

	fmt.Printf("🚀 Streaming %s to %s...\n", scriptPath, hostAlias)

	// 2. Set up the SSH command
	// 'bash -s' tells the remote bash session to read commands from standard input
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		hostAlias,
		"bash -s", 
	}
	cmd := exec.Command("ssh", sshArgs...)

	// 3. Inject the correct SSH Agent Socket!
	sockPath := getSocketForHost(hostAlias, cfg)
	if sockPath != "" {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("SSH_AUTH_SOCK=%s", sockPath))
	}

	// 4. Open the local script file
	file, err := os.Open(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to open script: %v", err)
	}
	defer file.Close()

	// 5. Wire up the inputs and outputs
	// Stdin gets the file content. Stdout/Stderr go straight to your Mac's terminal.
	cmd.Stdin = file
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 6. Execute!
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script execution failed: %v", err)
	}

	fmt.Println("✅ Execution complete!")
	return nil
}
