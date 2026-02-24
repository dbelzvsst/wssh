package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type hostItem struct {
	host SearchableHost
}

func (i hostItem) Title() string { return i.host.Alias }
func (i hostItem) Description() string {
	return fmt.Sprintf("%s | Group: %s", i.host.Hostname, i.host.GroupName)
}
func (i hostItem) FilterValue() string { return i.host.SearchIndex }

type model struct {
	list     list.Model
	choice   *SearchableHost
	quitting bool
	originalItems []list.Item // Keep track of the default YAML order
	sortMode      string      // "default" or "recent"	
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+r":
			var cmd tea.Cmd
			if m.sortMode == "default" {
				m.sortMode = "recent"
				m.list.Title = "wssh - Recent Hosts (ctrl+r to revert | ctrl+p push .tgz)"
				
				recentAliases := GetRecentHosts()
				var newItems []list.Item
				added := make(map[string]bool)

				// 1. Put the recent items at the top
				for _, alias := range recentAliases {
					for _, item := range m.originalItems {
						if hi, ok := item.(hostItem); ok && hi.host.Alias == alias {
							newItems = append(newItems, item)
							added[alias] = true
							break
						}
					}
				}

				// 2. Append all the remaining hosts that aren't in the history yet
				for _, item := range m.originalItems {
					if hi, ok := item.(hostItem); ok && !added[hi.host.Alias] {
						newItems = append(newItems, item)
					}
				}
				
				cmd = m.list.SetItems(newItems)
			} else {
				// Revert to default YAML order
				m.sortMode = "default"
				m.list.Title = "wssh - Select a Host (ctrl+r for recent)"
				cmd = m.list.SetItems(m.originalItems)
			}
			return m, cmd
		case "ctrl+p":
			selectedItem, ok := m.list.SelectedItem().(hostItem)
			if !ok {
				return m, nil
			}
			targetHost := selectedItem.host

			// Clear the screen for aesthetics
			c := exec.Command("clear")
			c.Stdout = os.Stdout
			c.Run()

			// Call our native Go menu!
			cmd := tea.ExecProcess(exec.Command("wssh", "pushmenu", targetHost.Alias), func(err error) tea.Msg {
				return nil
			})
			return m, cmd
		case "enter":
			i, ok := m.list.SelectedItem().(hostItem)
			if ok {
				m.choice = &i.host
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.choice != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(fmt.Sprintf("Connecting to %s...\n", m.choice.Alias))
	}
	if m.quitting {
		return "Goodbye!\n"
	}
	return docStyle.Render(m.list.View())
}

// --- NEW BOOLEAN FILTER LOGIC ---

// customFilter overrides the default fuzzy finder
func customFilter(term string, targets []string) []list.Rank {
	var ranks []list.Rank
	for i, target := range targets {
		if matchBoolean(term, target) {
			// If it matches our boolean logic, we append it to the results.
			// (Note: We leave MatchedIndices empty because calculating exact letter
			// highlighting for complex boolean queries is overly complex).
			ranks = append(ranks, list.Rank{Index: i})
		}
	}
	return ranks
}

// matchBoolean tokenizes the query and evaluates OR groups
func matchBoolean(query, target string) bool {
	if strings.TrimSpace(query) == "" {
		return true // Show all if search is empty
	}

	target = strings.ToLower(target)
	tokens := strings.Fields(query) // Split by spaces

	var orGroups [][]string
	var currentGroup []string

	// Split the tokens into groups separated by "OR"
	for _, t := range tokens {
		if strings.ToUpper(t) == "OR" {
			orGroups = append(orGroups, currentGroup)
			currentGroup = nil
		} else {
			currentGroup = append(currentGroup, t)
		}
	}
	orGroups = append(orGroups, currentGroup)

	// If ANY of the OR groups return true, the whole query is true
	for _, group := range orGroups {
		if matchAndGroup(group, target) {
			return true
		}
	}
	return false
}

// matchAndGroup evaluates AND and NOT conditions within a single OR group
func matchAndGroup(tokens []string, target string) bool {
	if len(tokens) == 0 {
		return false
	}

	negateNext := false

	for _, token := range tokens {
		upper := strings.ToUpper(token)

		// Explicit "AND" is ignored because spaces are implicit ANDs
		if upper == "AND" {
			continue
		}

		// If we see "NOT", flag the next token to be negated
		if upper == "NOT" {
			negateNext = true
			continue
		}

		// Evaluate the search term
		matched := strings.Contains(target, strings.ToLower(token))

		if negateNext {
			if matched {
				return false // It contained a word it was NOT supposed to
			}
			negateNext = false // Reset the flag
		} else {
			if !matched {
				return false // It missed a word it WAS supposed to have
			}
		}
	}
	return true // All AND/NOT conditions were satisfied!
}

// --- END BOOLEAN LOGIC ---

func RunTUI(searchableHosts []SearchableHost) *SearchableHost {
	items := make([]list.Item, len(searchableHosts))
	for i, h := range searchableHosts {
		items[i] = hostItem{host: h}
	}

	m := model{
		list: list.New(items, list.NewDefaultDelegate(), 0, 0),
		originalItems: items,
		sortMode:      "default",
	}
	m.list.Title = "wssh - Select a Host (ctrl+r recent | ctrl+p push .tgz)"	

	// Inject our custom boolean filter into the list model!
	m.list.Filter = customFilter

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(model); ok && m.choice != nil {
		return m.choice
	}
	return nil
}
