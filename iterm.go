package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const sshRetryDelaySeconds = 5

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func escapeAppleScriptString(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\\"`)
	return escaped
}

func buildRetryingSSHCommand(baseSshCmd string) string {
	return fmt.Sprintf("while true; do %s; ssh_exit=$?; if [ $ssh_exit -eq 0 ]; then break; fi; if [ $ssh_exit -eq 130 ] || [ $ssh_exit -eq 143 ]; then echo 'Retry interrupted.'; break; fi; echo 'Connection failed. Retrying in %d seconds... (Ctrl+C to stop)'; sleep %d; done", baseSshCmd, sshRetryDelaySeconds, sshRetryDelaySeconds)
}

func ExecuteAppleScript(script string) error {
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript error: %s\nOutput: %s", err, string(output))
	}
	return nil
}

// LaunchLayout handles layouts, applies profiles, and wraps the command in logging/flags
func LaunchLayout(host SearchableHost, layout string, cfg *Config, retry bool) error {
	WarnKeyExpiration(host.Alias, cfg)
	sshArgs := ""

	// 1. Ignore Key Changes (Default is true if missing from YAML)
	ignoreKeys := true
	if cfg.Settings.IgnoreKeyChanges != nil {
		ignoreKeys = *cfg.Settings.IgnoreKeyChanges
	}
	if ignoreKeys {
		sshArgs += "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "
	}

	// The base SSH command (without the logging wrapper)
	baseSshCmd := fmt.Sprintf("ssh %s%s", sshArgs, host.Alias)
	sshCmdForPane := baseSshCmd
	if retry {
		sshCmdForPane = buildRetryingSSHCommand(baseSshCmd)
	}

	// 2. Fallback to iTerm's default profile if empty
	profileStr := `default profile`
	if host.Profile != "" {
		profileStr = fmt.Sprintf(`profile "%s"`, host.Profile)
	}

	// 3. Helper function to generate a unique command per pane
	getCmdForPane := func(paneIndex int) string {
		shellWrappedCmd := fmt.Sprintf("sh -lc %s", shellSingleQuote(sshCmdForPane))

		if !host.LogSession {
			return escapeAppleScriptString(shellWrappedCmd)
		}

		homeDir, _ := os.UserHomeDir()
		logDir := filepath.Join(homeDir, "wssh_logs")
		os.MkdirAll(logDir, 0700)

		timestamp := time.Now().Format("2006-01-02_15-04-05")

		logFile := filepath.Join(logDir, fmt.Sprintf("%s_%s_pane%d.log", host.Alias, timestamp, paneIndex))

		// Wrap the shell command in the native macOS 'script' utility
		loggedCmd := fmt.Sprintf("script -q %s %s", shellSingleQuote(logFile), shellWrappedCmd)
		return escapeAppleScriptString(loggedCmd)
	}

	var script string

	// 4. Generate the AppleScript based on the requested layout
	switch layout {
	case "2h": // 2 Horizontal
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					set newTab to (create tab with %s command "%s")
					tell newTab
						set pane1 to current session
						tell pane1
							set pane2 to (split horizontally with %s command "%s")
						end tell
					end tell
				end tell
			end tell`, profileStr, getCmdForPane(1), profileStr, getCmdForPane(2))

	case "2v": // 2 Vertical
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					set newTab to (create tab with %s command "%s")
					tell newTab
						set pane1 to current session
						tell pane1
							set pane2 to (split vertically with %s command "%s")
						end tell
					end tell
				end tell
			end tell`, profileStr, getCmdForPane(1), profileStr, getCmdForPane(2))

	case "3h": // 3 Horizontal
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					set newTab to (create tab with %s command "%s")
					tell newTab
						set pane1 to current session
						tell pane1
							set pane2 to (split horizontally with %s command "%s")
						end tell
						tell pane2
							set pane3 to (split horizontally with %s command "%s")
						end tell
					end tell
				end tell
			end tell`, profileStr, getCmdForPane(1), profileStr, getCmdForPane(2), profileStr, getCmdForPane(3))

	case "3v": // 3 Vertical
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					set newTab to (create tab with %s command "%s")
					tell newTab
						set pane1 to current session
						tell pane1
							set pane2 to (split vertically with %s command "%s")
						end tell
						tell pane2
							set pane3 to (split vertically with %s command "%s")
						end tell
					end tell
				end tell
			end tell`, profileStr, getCmdForPane(1), profileStr, getCmdForPane(2), profileStr, getCmdForPane(3))

	case "4g": // 2x2 Grid
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					set newTab to (create tab with %s command "%s")
					tell newTab
						set pane1 to current session
						tell pane1
							set pane2 to (split vertically with %s command "%s")
						end tell
						tell pane1
							set pane3 to (split horizontally with %s command "%s")
						end tell
						tell pane2
							set pane4 to (split horizontally with %s command "%s")
						end tell
					end tell
				end tell
			end tell`, profileStr, getCmdForPane(1), profileStr, getCmdForPane(2), profileStr, getCmdForPane(3), profileStr, getCmdForPane(4))

	default: // "single" or unrecognized input
		script = fmt.Sprintf(`tell application "iTerm2"
				tell current window
					create tab with %s command "%s"
				end tell
			end tell`, profileStr, getCmdForPane(1))
	}

	// 5. Log the successful execution to your local history file
	err := LogConnection(host.Alias)
	if err != nil {
		fmt.Printf("Warning: Failed to log connection history: %v\n", err)
	}

	return ExecuteAppleScript(script)
}

// SendMacro injects text into the currently active iTerm pane and executes it
func SendMacro(command string) error {
	// Escape double quotes and backslashes so AppleScript doesn't break
	safeCmd := strings.ReplaceAll(command, `\`, `\\`)
	safeCmd = strings.ReplaceAll(safeCmd, `"`, `\"`)

	// The AppleScript targets the 'current session' and writes the text.
	// We add \n at the end so it executes the command automatically.
	script := fmt.Sprintf(`tell application "iTerm2"
		tell current window
			tell current session
				write text "%s"
			end tell
		end tell
	end tell`, safeCmd)

	return ExecuteAppleScript(script)
}

// LaunchInitLayout creates a helper workspace tab:
// split vertically, then split the new pane horizontally, and cd in each right-side pane.
func LaunchInitLayout(cfg *Config) error {
	topDir := cfg.Settings.InitLayout.TopDir
	bottomDir := cfg.Settings.InitLayout.BottomDir

	if topDir == "" && bottomDir == "" {
		return fmt.Errorf("init_layout.top_dir and init_layout.bottom_dir are not set in ~/.wssh.yaml")
	}

	script := fmt.Sprintf(`tell application "iTerm2"
		tell current window
			set newTab to (create tab with default profile)
			tell newTab
				set pane1 to current session
				tell pane1
					set pane2 to (split vertically with default profile)
				end tell
				tell pane2
					write text "cd %s"
					set pane3 to (split horizontally with default profile)
				end tell
				tell pane3
					write text "cd %s"
				end tell
			end tell
		end tell
	end tell`, escapeAppleScriptString(topDir), escapeAppleScriptString(bottomDir))

	return ExecuteAppleScript(script)
}
