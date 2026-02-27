package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// FindHosts takes a slice of search terms and returns all hosts that match ALL terms (Logical AND)
func FindHosts(terms []string, allHosts []SearchableHost) []SearchableHost {
	var matches []SearchableHost

	// 1. Pre-process and compile search terms once to save performance
	type parsedTerm struct {
		literal string
		re      *regexp.Regexp
	}

	var parsed []parsedTerm
	for _, t := range terms {
		p := parsedTerm{literal: strings.ToLower(t)}
		
		// If the term contains regex-like characters, try to compile it
		if strings.ContainsAny(t, "[]*?^$|") {
			// Add (?i) to make the regex case-insensitive
			if re, err := regexp.Compile("(?i)" + t); err == nil {
				p.re = re
			}
		}
		parsed = append(parsed, p)
	}

	// 2. Loop through hosts
	for _, host := range allHosts {
		target := strings.ToLower(host.SearchIndex)

		match := true
		for _, pt := range parsed {
			if pt.re != nil {
				// Use Regex Match
				if !pt.re.MatchString(target) {
					match = false
					break
				}
			} else {
				// Fallback to literal Substring Match
				if !strings.Contains(target, pt.literal) {
					match = false
					break
				}
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