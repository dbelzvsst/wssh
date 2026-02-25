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
	choices  []SearchableHost
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
				m.list.Title = "wssh - Select a Host (ctrl+r recent | ctrl+p push | ctrl+a connect all)"
				
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
		case "ctrl+a":
			// Get all currently visible (filtered) items
			visibleItems := m.list.VisibleItems()
			if len(visibleItems) > 0 {
				for _, item := range visibleItems {
					if i, ok := item.(hostItem); ok {
						m.choices = append(m.choices, i.host)
					}
				}
			}
			return m, tea.Quit
			
		case "enter":
			i, ok := m.list.SelectedItem().(hostItem)
			if ok {
				m.choices = []SearchableHost{i.host} // Assign as a slice of 1
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
	if len(m.choices) == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(fmt.Sprintf("Connecting to %s...\n", m.choices[0].Alias))
	} else if len(m.choices) > 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(fmt.Sprintf("Connecting to %d hosts...\n", len(m.choices)))
	}
	if m.quitting {
		return "Goodbye!\n"
	}
	return docStyle.Render(m.list.View())
}

// customFilter overrides the default fuzzy finder to match our CLI AND-logic
func customFilter(term string, targets []string) []list.Rank {
	var ranks []list.Rank
	
	// Split the search bar input into individual terms (e.g. "dev" "web")
	terms := strings.Fields(strings.ToLower(term))

	for i, target := range targets {
		targetLower := strings.ToLower(target)
		
		// Assume match, then try to disprove it (Logical AND)
		match := true
		for _, t := range terms {
			if !strings.Contains(targetLower, t) {
				match = false
				break // Failed the AND check, move to the next host
			}
		}

		if match {
			// If it passes, append it. (We leave MatchedIndexes empty to keep rendering fast)
			ranks = append(ranks, list.Rank{Index: i})
		}
	}
	return ranks
}

func RunTUI(searchableHosts []SearchableHost) []SearchableHost {
	items := make([]list.Item, len(searchableHosts))
	for i, h := range searchableHosts {
		items[i] = hostItem{host: h}
	}

	m := model{
		list: list.New(items, list.NewDefaultDelegate(), 0, 0),
		originalItems: items,
		sortMode:      "default",
	}
	m.list.Title = "wssh - Select a Host (ctrl+r recent | ctrl+p push | ctrl+a connect all)"	

	// Inject our custom filter into the list model!
	m.list.Filter = customFilter	
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(model); ok && len(m.choices) > 0 {
		return m.choices
	}
	return nil
}
