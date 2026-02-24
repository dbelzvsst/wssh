package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const historyFileName = ".wssh_history"

// LogConnection appends a successful connection to the history file
func LogConnection(alias string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	historyPath := filepath.Join(homeDir, historyFileName)

	// Open file in append mode, create it if it doesn't exist
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Format: 2026-02-23T14:30:00-08:00,spprod-vwa-bsa-01
	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("%s,%s\n", timestamp, alias)

	_, err = f.WriteString(entry)
	return err
}

// PrintHistory reads the file and outputs the most recent connections
func PrintHistory(limit int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	historyPath := filepath.Join(homeDir, historyFileName)

	file, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No connection history found yet.")
			return nil
		}
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) == 0 {
		fmt.Println("History is empty.")
		return nil
	}

	fmt.Println("--- Recent Connections ---")
	
	// Print in reverse order (newest first), up to the limit
	count := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if limit > 0 && count >= limit {
			break
		}
		
		parts := strings.SplitN(lines[i], ",", 2)
		if len(parts) == 2 {
			t, _ := time.Parse(time.RFC3339, parts[0])
			// Print nicely formatted: 02/23 14:30 | prod-east-01
			fmt.Printf("%s | %s\n", t.Format("01/02 15:04"), parts[1])
		}
		count++
	}

	return scanner.Err()
}

// GetRecentHosts returns a deduplicated list of recent host aliases (newest first)
func GetRecentHosts() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	historyPath := filepath.Join(homeDir, historyFileName)

	file, err := os.Open(historyPath)
	if err != nil {
		return nil // It's okay if history doesn't exist yet
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	seen := make(map[string]bool)
	var recent []string

	// Read backwards so newest is first
	for i := len(lines) - 1; i >= 0; i-- {
		parts := strings.SplitN(lines[i], ",", 2)
		if len(parts) == 2 {
			alias := parts[1]
			if !seen[alias] {
				seen[alias] = true
				recent = append(recent, alias)
			}
		}
	}
	return recent
}

