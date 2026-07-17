// Package tui renders the QUANTUM_LOG terminal dashboard from injected data.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type OverviewData struct {
	TotalTokens            int64
	AllocatedCostUSDMicros int64
	Projects               []Project
	Tasks                  []Task
	Integrity              Integrity
}

type Project struct {
	Name          string
	Slug          string
	LocationCount int64
	TagCount      int64
}

type Task struct {
	ProjectSlug string
	Title       string
	Type        string
	Status      string
}

type Integrity struct {
	SQLite string
	Ledger string
}

type screen int

const (
	overviewScreen screen = iota
	projectsScreen
	tasksScreen
	integrityScreen
)

var screens = []string{"Overview", "Projects", "Tasks", "Integrity"}

type styles struct {
	header  lipgloss.Style
	active  lipgloss.Style
	warning lipgloss.Style
}

type Model struct {
	data   OverviewData
	screen screen
	width  int
	help   bool
	styles styles
}

func New(data OverviewData, noColor bool) Model {
	model := Model{data: data, width: 80}
	if !noColor {
		model.styles = styles{
			header:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
			active:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
			warning: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		}
	}
	return model
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
	case tea.KeyMsg:
		switch message.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help = !m.help
		case "esc":
			m.screen = overviewScreen
		case "left", "h", "shift+tab":
			m.screen = (m.screen + screen(len(screens)) - 1) % screen(len(screens))
		case "right", "l", "tab":
			m.screen = (m.screen + 1) % screen(len(screens))
		case "1":
			m.screen = overviewScreen
		case "2":
			m.screen = projectsScreen
		case "3":
			m.screen = tasksScreen
		case "4":
			m.screen = integrityScreen
		}
	}
	return m, nil
}

func (m Model) View() string {
	var view strings.Builder
	view.WriteString(m.styles.header.Render("QUANTUM_LOG terminal dashboard"))
	view.WriteString("\n")
	view.WriteString(m.navigation())
	view.WriteString("\n\n")
	if m.width > 0 && m.width < 60 {
		view.WriteString(m.styles.warning.Render("Terminal too narrow: use at least 60 columns for readable content."))
		view.WriteString("\n\n")
	} else if m.width > 0 && m.width < 80 {
		view.WriteString("Compact layout: use at least 80 columns for full details.\n\n")
	}
	switch m.screen {
	case overviewScreen:
		view.WriteString(m.overviewView())
	case projectsScreen:
		view.WriteString(m.projectsView())
	case tasksScreen:
		view.WriteString(m.tasksView())
	case integrityScreen:
		view.WriteString(m.integrityView())
	}
	view.WriteString("\n\n")
	if m.help {
		view.WriteString("Help: Left/Right or Tab change screen. 1-4 select screen. Esc returns to Overview. ? closes help. q or Ctrl+C quits.")
	} else {
		view.WriteString("Press ? for keyboard help. All screens use text labels; color is optional.")
	}
	return view.String()
}

func (m Model) navigation() string {
	items := make([]string, len(screens))
	for index, name := range screens {
		label := fmt.Sprintf("%d %s", index+1, name)
		if screen(index) == m.screen {
			items[index] = m.styles.active.Render("[Current: " + label + "]")
		} else {
			items[index] = label
		}
	}
	return strings.Join(items, " | ")
}

func (m Model) overviewView() string {
	return fmt.Sprintf("Overview\nTotal observed tokens: %d\nAllocated cost: %s\nRegistered projects: %d\nTasks: %d", m.data.TotalTokens, formatUSD(m.data.AllocatedCostUSDMicros), len(m.data.Projects), len(m.data.Tasks))
}

func (m Model) projectsView() string {
	if len(m.data.Projects) == 0 {
		return "Projects\nNo registered projects. Use qlog project register to add one."
	}
	var view strings.Builder
	view.WriteString("Projects")
	for _, project := range m.data.Projects {
		if m.compact() {
			fmt.Fprintf(&view, "\n- %s (%s)", project.Name, project.Slug)
			continue
		}
		fmt.Fprintf(&view, "\nProject: %s\nSlug: %s\nLocations: %d\nTags: %d", project.Name, project.Slug, project.LocationCount, project.TagCount)
	}
	return view.String()
}

func (m Model) tasksView() string {
	if len(m.data.Tasks) == 0 {
		return "Tasks\nNo tasks recorded. Use qlog task start to add one."
	}
	var view strings.Builder
	view.WriteString("Tasks")
	for _, task := range m.data.Tasks {
		if m.compact() {
			fmt.Fprintf(&view, "\n- %s: %s", task.Status, task.Title)
			continue
		}
		fmt.Fprintf(&view, "\nTask: %s\nProject: %s\nType: %s\nStatus: %s", task.Title, task.ProjectSlug, task.Type, task.Status)
	}
	return view.String()
}

func (m Model) integrityView() string {
	return fmt.Sprintf("Integrity\nSQLite database: %s\nLedger hash chains: %s", m.data.Integrity.SQLite, m.data.Integrity.Ledger)
}

func (m Model) compact() bool { return m.width > 0 && m.width < 80 }

func formatUSD(micros int64) string {
	sign := ""
	if micros < 0 {
		sign = "-"
		micros = -micros
	}
	return fmt.Sprintf("%s$%d.%06d", sign, micros/1_000_000, micros%1_000_000)
}
