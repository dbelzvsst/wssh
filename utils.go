package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FindHosts takes a slice of search terms and returns all hosts that match ALL terms (Logical AND)
func FindHosts(terms []string, allHosts []SearchableHost) []SearchableHost {
	var matches []SearchableHost

	// Convert all search terms to lowercase once
	var lowerTerms []string
	for _, t := range terms {
		lowerTerms = append(lowerTerms, strings.ToLower(t))
	}

	for _, host := range allHosts {
		// Using SearchIndex ensures we match against Group, Tags, Alias, and Hostname
		target := strings.ToLower(host.SearchIndex)

		match := true
		for _, term := range lowerTerms {
			if !strings.Contains(target, term) {
				match = false
				break // Failed the AND check
			}
		}

		if match {
			matches = append(matches, host)
		}
	}

	return matches
}

// ConfirmExecution prompts the user for confirmation if there is more than 1 matching host.
// It automatically returns true if there is exactly 1 match, and false if there are 0.
func ConfirmExecution(hosts []SearchableHost, action string) bool {
	if len(hosts) == 0 {
		fmt.Println("❌ No hosts matched the search criteria.")
		return false
	}

	// If exactly one host matches, proceed without prompting
	if len(hosts) == 1 {
		return true
	}

	// More than one host matched, ask for confirmation
	fmt.Printf("--- %s on %d Hosts ---\n", action, len(hosts))
	for _, h := range hosts {
		fmt.Printf("  - %-20s [Group: %s]\n", h.Alias, h.GroupName)
	}

	fmt.Print("\nProceed? (y/N): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response := strings.ToLower(strings.TrimSpace(scanner.Text()))

	return response == "y" || response == "yes"
}


// ListHosts lists the matching hosts
func ListHosts(hosts []SearchableHost) {
	if len(hosts) == 0 {
		fmt.Println("❌ No hosts matched the search criteria.")
		return
	}

	// If exactly one host matches, proceed without prompting
	if len(hosts) == 1 {
		return
	}

	// More than one host matched, ask for confirmation
	fmt.Printf("--- Matching Hosts (%d) ---\n", len(hosts))
	for _, h := range hosts {
		fmt.Printf("  - %-20s [Group: %s]\n", h.Alias, h.GroupName)
	}

	return
}