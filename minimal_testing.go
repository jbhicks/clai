package main

import (
	"bufio"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
)

// UIComponent is an interface for prototyping any Bubble Tea component
// It must implement Update, View, and optionally Init
// This allows you to swap in any component for rapid prototyping
// Example usage: see README.md

type UIComponent interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (UIComponent, tea.Cmd)
	View() string
}

type model struct {
	component UIComponent
}

func initialModel(component UIComponent) model {
	return model{component: component}
}

func (m model) Init() tea.Cmd {
	return m.component.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	newComponent, cmd := m.component.Update(msg)
	m.component = newComponent

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}
	return m, cmd
}

var boxStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("63")).
	Padding(0, 1)

func (m model) View() string {
	return fmt.Sprintf(
		"Component Output:\n%s\nStatus: Prototyping UI. Press q to quit.",
		boxStyle.Render(m.component.View()),
	)
}

// Example: To prototype a component, define it here and pass to initialModel
// See README.md for details

func main() {
	// Replace ExampleComponent() with your own component for prototyping
	p := tea.NewProgram(initialModel(ExampleComponent()))
	if err := p.Start(); err != nil {
		panic(err)
	}
}

// DebugLogComponent displays the contents of debug.log

type DebugLogComponent struct {
	lines []string
	pos   int // scroll position
}

func (d *DebugLogComponent) Init() tea.Cmd {
	file, err := os.Open("debug.log")
	if err != nil {
		d.lines = []string{"Could not open debug.log: " + err.Error()}
		return nil
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		lines = append(lines, "Error reading debug.log: "+err.Error())
	}
	d.lines = lines
	return nil
}

func (d *DebugLogComponent) Update(msg tea.Msg) (UIComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "up" && d.pos > 0 {
			d.pos--
		}
		if msg.String() == "down" && d.pos < len(d.lines)-10 {
			d.pos++
		}
	}
	return d, nil
}

func (d *DebugLogComponent) View() string {
	if len(d.lines) == 0 {
		return "debug.log is empty."
	}
	// Show up to 10 lines at a time
	start := d.pos
	end := d.pos + 10
	if end > len(d.lines) {
		end = len(d.lines)
	}
	view := ""
	for i := start; i < end; i++ {
		view += d.lines[i] + "\n"
	}
	return view + fmt.Sprintf("\nShowing lines %d-%d of %d. Use up/down to scroll.", start+1, end, len(d.lines))
}

func ExampleComponent() UIComponent {
	return &DebugLogComponent{}
}

type exampleComponentModel struct {
	counter int
}

func (e *exampleComponentModel) Init() tea.Cmd {
	return nil
}

func (e *exampleComponentModel) Update(msg tea.Msg) (UIComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "space" {
			e.counter++
		}
	}
	return e, nil
}

func (e *exampleComponentModel) View() string {
	return fmt.Sprintf("Counter: %d (press space to increment)", e.counter)
}
