package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// --- YAML Data Structures ---

type Row struct {
	Panes int `yaml:"panes"`
}

type Layout struct {
	Description string `yaml:"description"`
	Rows        []Row  `yaml:"rows"`
}

type Host struct {
	Alias    string   `yaml:"alias"`
	Hostname string   `yaml:"hostname"`
	Tags     []string `yaml:"tags"`
}

type Group struct {
	Name    string   `yaml:"name"`
	Tags    []string `yaml:"tags,omitempty"`
	Profile string   `yaml:"profile,omitempty"`
	LogSession bool     `yaml:"log_session,omitempty"`
	Hosts   []Host   `yaml:"hosts"`
}

// Config represents the entire ~/.wssh.yaml file
type AgentEnv struct {
	Sock string `yaml:"sock"`
	Key  string `yaml:"key"`
}

type Settings struct {
	AgentExpirationHours float64             `yaml:"agent_expiration_hours"`
	AuthCheckEnv         string              `yaml:"auth_check_env"`
	IgnoreKeyChanges     *bool               `yaml:"ignore_key_changes"`	
	SSHAgentEnvs         map[string]AgentEnv `yaml:"ssh_agent_envs"`
	CaptureCommand       string              `yaml:"capture_command"`
}

type Config struct {
	Settings Settings               `yaml:"settings,omitempty"`
	Payloads map[string]string      `yaml:"payloads"`
	Layouts  map[string]interface{} `yaml:"layouts,omitempty"`
	Macros   map[string]string      `yaml:"macros,omitempty"`
	Groups   []Group                `yaml:"groups"`
}

// --- Application Data Structures ---
type SearchableHost struct {
	Alias       string
	Hostname    string
	GroupName   string
	SearchIndex string
	Profile     string
	LogSession  bool
}

// SearchableHost is the flattened struct we will pass to the TUI for fuzzy finding.

// LoadConfig reads the YAML file, parses it, and builds the flat search index.
func LoadConfig(path string) (*Config, []SearchableHost, error) {
	// 1. Read the raw file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. Unmarshal YAML into our Go struct
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	// 3. Build the flattened search index
	var searchableHosts []SearchableHost

	for _, group := range cfg.Groups {
		for _, host := range group.Hosts {
			// Combine group tags, host tags, the group name, and the host alias.
			// This ensures typing *any* of these words will match the host.
			allKeywords := append([]string{group.Name, host.Alias, host.Hostname}, group.Tags...)
			allKeywords = append(allKeywords, host.Tags...)

			// Convert everything to lowercase to make searching case-insensitive
			for i, word := range allKeywords {
				allKeywords[i] = strings.ToLower(word)
			}

			// Join them into a single space-separated string:
			// e.g., "development web-dev-uswest-01 dev sandbox us-west cluster-a web"
			searchIndex := strings.Join(allKeywords, " ")

			searchableHosts = append(searchableHosts, SearchableHost{
				Alias:       host.Alias,
				Hostname:    host.Hostname,
				GroupName:   group.Name,
				SearchIndex: searchIndex,
				Profile:     group.Profile,
				LogSession:  group.LogSession,
			})
		}
	}

	return &cfg, searchableHosts, nil
}
