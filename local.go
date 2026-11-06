package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func LaunchLocal(host SearchableHost, cfg *Config, retry bool) error {
	WarnKeyExpiration(host.Alias, cfg)

	sshArgs := []string{}
	ignoreKeys := true
	if cfg.Settings.IgnoreKeyChanges != nil {
		ignoreKeys = *cfg.Settings.IgnoreKeyChanges
	}
	if ignoreKeys {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	sshArgs = append(sshArgs, host.Alias)

	if err := LogConnection(host.Alias); err != nil {
		fmt.Printf("Warning: Failed to log connection history: %v\n", err)
	}

	for {
		cmd := exec.Command("ssh", sshArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			return nil
		}

		if !retry {
			return err
		}

		if isInterruptExit(err) {
			fmt.Println("Retry interrupted.")
			return nil
		}

		fmt.Printf("Connection failed. Retrying in %d seconds... (Ctrl+C to stop)\n", sshRetryDelaySeconds)
		time.Sleep(time.Duration(sshRetryDelaySeconds) * time.Second)
	}
}

func isInterruptExit(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}

	if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 143 {
		return true
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	if !status.Signaled() {
		return false
	}

	sig := status.Signal()
	return sig == os.Interrupt || sig == syscall.SIGTERM
}
