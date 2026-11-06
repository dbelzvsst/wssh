package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLastHostCLI(t *testing.T) {
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "wssh")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte("#!/bin/sh\nprintf 'SSH: %s\\n' \"$*\"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		args       []string
		history    string
		want       string
		wantError  bool
		mustNotSSH bool
	}{
		{name: "subcommand", args: []string{"last", "--local"}, history: "2026-11-06T12:00:00Z,prod-db\n", want: "SSH: prod-db"},
		{name: "flag", args: []string{"--last", "--local"}, history: "2026-11-06T11:00:00Z,old-host\n2026-11-06T12:00:00Z,prod-db\n\n", want: "SSH: prod-db"},
		{name: "adhoc", args: []string{"--last", "--local", "--retry"}, history: "2026-11-06T12:00:00Z,adhoc-host\n", want: "SSH: adhoc-host"},
		{name: "flag layout", args: []string{"--last", "4g", "--local"}, history: "2026-11-06T12:00:00Z,prod-db\n", want: "Reconnecting to prod-db with layout '4g'"},
		{name: "subcommand layout", args: []string{"last", "2v", "--local"}, history: "2026-11-06T12:00:00Z,prod-db\n", want: "Reconnecting to prod-db with layout '2v'"},
		{name: "missing history", args: []string{"--last", "--local"}, want: "no connection history found yet", wantError: true, mustNotSSH: true},
		{name: "empty history", args: []string{"last", "--local"}, history: "\n", want: "history is empty", wantError: true, mustNotSSH: true},
		{name: "invalid history", args: []string{"--last", "--local"}, history: "broken\n", want: "invalid history entry", wantError: true, mustNotSSH: true},
		{name: "too many flag args", args: []string{"--last", "2v", "extra", "--local"}, want: "accepts at most 1 arg(s)", wantError: true, mustNotSSH: true},
		{name: "too many subcommand args", args: []string{"last", "2v", "extra", "--local"}, want: "accepts at most 1 arg(s)", wantError: true, mustNotSSH: true},
		{name: "host completion", args: []string{"__complete", "prod"}, want: "prod-db\tProduction", mustNotSSH: true},
		{name: "flag layout completion", args: []string{"__complete", "--last", "2"}, want: "2v\tTwo vertical panes", mustNotSSH: true},
		{name: "subcommand layout completion", args: []string{"__complete", "last", "4"}, want: "4g\t2x2 Grid", mustNotSSH: true},
		{name: "zsh completion script", args: []string{"completion", "zsh"}, want: "#compdef wssh", mustNotSSH: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			config := "settings:\n  ignore_key_changes: false\ngroups:\n  - name: Production\n    hosts:\n      - alias: prod-db\n"
			if err := os.WriteFile(filepath.Join(home, ".wssh.yaml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			if test.history != "" {
				if err := os.WriteFile(filepath.Join(home, historyFileName), []byte(test.history), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, test.args...)
			command.Env = append(os.Environ(), "HOME="+home, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := command.CombinedOutput()
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %v\n%s", err, test.wantError, output)
			}
			if !strings.Contains(string(output), test.want) {
				t.Errorf("output missing %q:\n%s", test.want, output)
			}
			if test.mustNotSSH && strings.Contains(string(output), "SSH:") {
				t.Errorf("unexpected SSH launch:\n%s", output)
			}
		})
	}
}
