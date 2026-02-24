package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// getSocketForHost dynamically determines the SSH_AUTH_SOCK by matching the host alias
// prefix against the defined ssh_agent_envs in the config.
func getSocketForHost(alias string, cfg *Config) string {
	// 1. Check for a specific environment prefix match (e.g., "dev", "prod")
	for envName, envData := range cfg.Settings.SSHAgentEnvs {
		// Skip the default key during the prefix loop
		if envName == "default" {
			continue
		}
		
		// If the alias (e.g., "dev-02") starts with "dev"
		if strings.HasPrefix(alias, envName) {
			return envData.Sock
		}
	}

	// 2. Fallback: If no prefix matched, use the "default" agent if configured
	if defaultEnv, exists := cfg.Settings.SSHAgentEnvs["default"]; exists {
		return defaultEnv.Sock
	}

	// 3. Absolute fallback: Return empty so SSH uses the system default
	return ""
}

// ResolveTargetNodes queries the JB to find its true FQDN and the dynamic target nodes
// Important - This was added because the easiest way to get to the data nodes is through the jumpbox,
// and the jumpbox knows the FQDN of the data nodes, which can change at any time. 
// This is a very specific use case, so not really applicable to the general use case. 
// TODO: find a way to make this optional for the generic cases or when jumpbox access isn't required.
func ResolveTargetNodes(jbAlias, nodeList string, cfg *Config) ([]string, string, error) {
	fmt.Printf("🔍 Connecting to %s to resolve true FQDN and node mappings...\n", jbAlias)

	sockPath := getSocketForHost(jbAlias, cfg)

	// Pull the command from the config, fallback to a safe default if empty
    remoteCmd := cfg.Settings.CaptureCommand
    if remoteCmd == "" {
        remoteCmd = "hostname -f" 
    }

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no", 
		"-o", "UserKnownHostsFile=/dev/null",
		jbAlias, 
		remoteCmd,
	}
	
	cmd := exec.Command("ssh", sshArgs...)
	
	// Inject the SSH_AUTH_SOCK environment variable!
	if sockPath != "" {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("SSH_AUTH_SOCK=%s", sockPath))
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("failed to execute remote commands on %s: %v", jbAlias, err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		return nil, "", fmt.Errorf("unexpected output from jumpbox: %s", out.String())
	}

	// 1. The first line is the true FQDN (from `hostname -f`)
	trueFQDN := strings.TrimSpace(lines[0])
	fmt.Printf("   📌 True Jumpbox FQDN: %s\n", trueFQDN)

	// Extract the domain, stripping out the routing subdomains if present
	domain := ""
	parts := strings.SplitN(trueFQDN, ".", 2)
	if len(parts) == 2 {
		domainPart := parts[1]
		if strings.HasPrefix(domainPart, "service.vsr.") {
			domainPart = strings.TrimPrefix(domainPart, "service.vsr.")
		}
		domain = "." + domainPart
	}

	// 2. The rest of the lines are the CSV output
	var targetFQDNs []string
	requestedNodes := strings.Split(nodeList, ",")

	// Start looping from index 1 (skipping the hostname output at index 0)
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		
		// Skip empty lines or the header row
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "VSERVER ID") {
			continue
		}

		cols := strings.Split(line, ",")
		if len(cols) < 7 {
			continue // Malformed row
		}

		vserverID := strings.TrimSpace(cols[0])
		hostName := strings.TrimSpace(cols[len(cols)-1])

		// Check if this row's ID is one of the nodes we asked for
		for _, reqNode := range requestedNodes {
			if vserverID == strings.TrimSpace(reqNode) {
				fullFQDN := hostName + domain
				targetFQDNs = append(targetFQDNs, fullFQDN)
				fmt.Printf("   ✅ Found Node %s -> %s\n", vserverID, fullFQDN)
			}
		}
	}

	if len(targetFQDNs) == 0 {
		return nil, "", fmt.Errorf("could not find requested nodes (%s)", nodeList)
	}

	// Return the FQDNs AND the socket path we used
	return targetFQDNs, sockPath, nil
}

// RunCapture streams live PCAP data from multiple hosts into a single Wireshark instance
func RunCapture(hosts []string, filter, sockPath string, cfg *Config) error {
	var wiresharkArgs []string
	wiresharkArgs = append(wiresharkArgs, "-k") // -k starts capturing immediately

	var fifos []string
	var sshCmds []*exec.Cmd

	// Defer cleanup: Ensure we kill the SSH streams and delete the pipes when Wireshark closes
	defer func() {
		for _, cmd := range sshCmds {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}
		for _, f := range fifos {
			os.Remove(f)
		}
	}()

	// 1. Create a Named Pipe (FIFO) for each host
	for i, host := range hosts {
		fifoPath := filepath.Join(os.TempDir(), fmt.Sprintf("wssh_cap_%s_%d.pcap", host, i))
		os.Remove(fifoPath) // Clean up just in case an old one is stuck
		
		if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
			return fmt.Errorf("failed to create fifo for %s: %v", host, err)
		}
		fifos = append(fifos, fifoPath)
		
		// Tell Wireshark to listen to this pipe
		wiresharkArgs = append(wiresharkArgs, "-i", fifoPath)
	}

	// 2. Start Wireshark FIRST (It must be running to open the read-end of the pipes)
	fmt.Printf("🦈 Launching Wireshark for %d host(s)...\n", len(hosts))
	wsCmd := exec.Command("wireshark", wiresharkArgs...)
	if err := wsCmd.Start(); err != nil {
		return fmt.Errorf("failed to start wireshark (ensure it is in your PATH): %v", err)
	}

	// 3. Start the parallel SSH streams
	for i, host := range hosts {
		remoteCmd := fmt.Sprintf("sudo tcpdump -U -w - %s", filter)
		sshArgs := []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", host, remoteCmd}
		
		cmd := exec.Command("ssh", sshArgs...)

		// Inject the exact same socket we used for the jumpbox!
		if sockPath != "" {
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, fmt.Sprintf("SSH_AUTH_SOCK=%s", sockPath))
		}

		// Open the pipe for writing...
		pipeFile, err := os.OpenFile(fifos[i], os.O_WRONLY, 0600)
		if err != nil {
			fmt.Printf("Warning: failed to open pipe for %s: %v\n", host, err)
			continue
		}
		cmd.Stdout = pipeFile
		cmd.Stderr = os.Stderr // Keep stderr attached to the terminal so you can see sudo prompts or errors

		if err := cmd.Start(); err != nil {
			fmt.Printf("Warning: failed to start SSH for %s: %v\n", host, err)
			continue
		}
		sshCmds = append(sshCmds, cmd)
	}

	// 4. Block and wait for you to close the Wireshark GUI
	wsCmd.Wait()
	fmt.Println("🛑 Wireshark closed. Cleaning up SSH streams and pipes...")
	return nil
}
