package main
// Moved to prototypes/minimal_testing.go
import (
	"bufio"
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"log"
	"os"
	"strings"
	"time"
)

// Logging helpers
func logDebug(msg string) {
	f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(f)
		log.Println(msg)
		f.Close()
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		logDebug("Tick: " + t.Format(time.RFC3339))
		return nil
	})
}

// Logging helpers
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

// TextInputComponent is a beautiful text input box using Bubbles and Lip Gloss
// Implements UIComponent
// Usage: pass NewTextInputComponent() to initialModel

type TextInputComponent struct {
	input textinput.Model
}

func NewTextInputComponent() *TextInputComponent {
	ti := textinput.New()
	ti.Placeholder = "Type something..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30
	// Style the input box
	ti.PromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("63")).
		Bold(true)
	ti.TextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))
	return &TextInputComponent{input: ti}
}

func (t *TextInputComponent) Init() tea.Cmd {
	return textinput.Blink
}

func (t *TextInputComponent) Update(msg tea.Msg) (UIComponent, tea.Cmd) {
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return t, cmd
}

func (t *TextInputComponent) View() string {
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Padding(0, 1)
	return bgStyle.Render(t.input.View())
}

// Composite UI: Chat + Logs + Status Bar

// Logging helpers
type appModel struct {
	chat      *ChatComponent
	logs      *DebugLogComponent
	statusMsg string
	width     int
	height    int
}

func newAppModel() appModel {
	return appModel{
		chat:      NewChatComponent(),
		logs:      &DebugLogComponent{},
		statusMsg: "Ready. Press q to quit.",
	}
}

func (m appModel) Init() tea.Cmd {
	logDebug("Init called")
	return tea.Batch(m.chat.Init(), m.logs.Init(), tickCmd())
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		logDebug(fmt.Sprintf("WindowSizeMsg received: width=%d, height=%d", m.width, m.height))
		// Propagate the window size message to sub-components if they need it
		// For now, we'll just update the main model's dimensions
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}
	// Update chat and logs
	newChat, chatCmd := m.chat.Update(msg)
	m.chat = newChat
	cmds = append(cmds, chatCmd)
	newLogs, logsCmd := m.logs.Update(msg)
	if logsComp, ok := newLogs.(*DebugLogComponent); ok {
		m.logs = logsComp
	}
	cmds = append(cmds, logsCmd)
	return m, tea.Batch(cmds...)
}

func (m appModel) View() string {
	// Define mainStyle first to use its frame size for content calculation
	mainStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2) // 1 vertical padding, 2 horizontal padding

	// Calculate inner content dimensions based on total window size minus mainStyle's frame
	innerContentWidth := m.width - mainStyle.GetHorizontalFrameSize()
	innerContentHeight := m.height - mainStyle.GetVerticalFrameSize()

	// Status bar height is 1 line + its vertical padding (0)
	statusBarHeight := 1 + lipgloss.NewStyle().Padding(0, 1).GetVerticalPadding()

	// Remaining height for chat/logs panes
	panesHeight := innerContentHeight - statusBarHeight

	// Each pane has a border (1 on each side) and a margin (2 for chat)
	paneBorderWidth := 2 // 1 on each side
	chatMarginRight := 2

	chatPaneWidth := (innerContentWidth - chatMarginRight - paneBorderWidth) / 2
	logsPaneWidth := innerContentWidth - chatPaneWidth - chatMarginRight - paneBorderWidth

	// Ensure minimum dimensions
	if chatPaneWidth < 10 {
		chatPaneWidth = 10
	}
	if logsPaneWidth < 10 {
		logsPaneWidth = 10
	}
	if panesHeight < 5 {
		panesHeight = 5
	}

	// Apply calculated dimensions to inner styles
	chatBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("36")).
		Width(chatPaneWidth).
		Height(panesHeight).
		MarginRight(chatMarginRight)

	logsBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("131")).
		Width(logsPaneWidth).
		Height(panesHeight)

	statusBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("229")).
		Width(innerContentWidth). // Status bar spans the inner content width
		Padding(0, 1)

	row := lipgloss.JoinHorizontal(lipgloss.Top,
		chatBoxStyle.Render(m.chat.View()),
		logsBoxStyle.Render(m.logs.View()),
	)
	statusBar := statusBarStyle.Render(m.statusMsg)

	// Render the combined content, then apply the mainStyle to it
	return mainStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, row, statusBar),
	)
}

// ChatComponent: text input + chat history

type ChatComponent struct {
	input   textinput.Model
	history []string
	width   int
	height  int
}

func NewChatComponent() *ChatComponent {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 34
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	return &ChatComponent{input: ti}
}

func (c *ChatComponent) Init() tea.Cmd {
	return textinput.Blink
}

func (c *ChatComponent) Update(msg tea.Msg) (*ChatComponent, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
	}
	c.input, cmd = c.input.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
		if val := c.input.Value(); val != "" {
			c.history = append(c.history, val)
			c.input.SetValue("")
		}
	}
	return c, cmd
}

func (c *ChatComponent) View() string {
	hist := ""
	// Explicitly set input box height to 1 line
	inputBoxHeight := 1

	// Calculate available height for history within the chat pane
	// c.height is the total height of the chat pane (including its border)
	// We need to subtract the chat pane's top/bottom borders (2 lines)
	// and the height of the input box (1 line)
	availableHistoryHeight := c.height - 2 - inputBoxHeight

	// Ensure availableHistoryHeight is not negative
	if availableHistoryHeight < 0 {
		availableHistoryHeight = 0
	}

	// Render history within available height
	historyLines := []string{}
	for i := len(c.history) - 1; i >= 0; i-- {
		historyLines = append(historyLines, "• "+c.history[i])
	}
	// Reverse to show newest at bottom
	for i, j := 0, len(historyLines)-1; i < j; i, j = i+1, j-1 {
		historyLines[i], historyLines[j] = historyLines[j], historyLines[i]
	}

	// Limit history to available lines
	if len(historyLines) > availableHistoryHeight {
		historyLines = historyLines[len(historyLines)-availableHistoryHeight:]
	}
	hist = strings.Join(historyLines, "\n")

	// Render the input box, ensuring its height is 1
	inputBox := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Height(inputBoxHeight). // Explicitly set height
		Render(c.input.View())

	// Render the history, ensuring its height is availableHistoryHeight
	historyView := lipgloss.NewStyle().
		Height(availableHistoryHeight).
		Render(hist)

	return lipgloss.JoinVertical(lipgloss.Left, historyView, inputBox)
}

// In main(), prototype the composite UI
func main() {
	p := tea.NewProgram(newAppModel())
	if err := p.Start(); err != nil {
		panic(err)
	}
}

// DebugLogComponent displays the contents of debug.log

type DebugLogComponent struct {
	lines  []string
	pos    int // scroll position
	width  int
	height int
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
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
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
	// Calculate available height for logs
	availableLogHeight := d.height - 1 // Account for the status line at the bottom

	// Show lines within available height
	start := d.pos
	end := d.pos + availableLogHeight
	if end > len(d.lines) {
		end = len(d.lines)
	}
	view := ""
	for i := start; i < end; i++ {
		view += d.lines[i] + "\n"
	}
	return lipgloss.NewStyle().Height(availableLogHeight).Render(view) +
		fmt.Sprintf("Showing lines %d-%d of %d. Use up/down to scroll.", start+1, end, len(d.lines))
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
