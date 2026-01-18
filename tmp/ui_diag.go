package main

import (
	"fmt"
	"strings"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/brittonhayes/glitter/theme"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	// Build a minimal theme similar to repo themes
	ui := glitter.NewUI(theme.Theme{Primary: theme.Primary{Background: lipgloss.Color("#1e1e2e"), Foreground: lipgloss.Color("#cdd6f4")}, Bright: theme.Bright{Blue: lipgloss.Color("#89b4fa"), Green: lipgloss.Color("#50fa7b"), White: lipgloss.Color("#ffffff")}})
	styles := GetThemeStyles(ui)

	termW := 120
	chatPaneWidth := int(float64(termW) * 0.8)
	rightColumnWidth := termW - chatPaneWidth

	// Simulate Chat inner view: small simple block
	chatInner := strings.Repeat("A", 80)
	chatViewInner := chatInner

	chatView := styles.MainPane.Width(chatPaneWidth).Render(chatViewInner)

	// Agent and log views
	agentView := styles.MainPane.Width(rightColumnWidth).Render("AGENT")
	logView := styles.MainPane.Width(rightColumnWidth).Render("LOG")
	rightInner := lipgloss.JoinVertical(lipgloss.Top, agentView, logView)
	rightView := styles.MainPane.Width(rightColumnWidth).Render(rightInner)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, chatView, rightView)

	printLineInfo("chatViewInner", chatViewInner)
	printLineInfo("chatView", chatView)
	printLineInfo("agentView", agentView)
	printLineInfo("logView", logView)
	printLineInfo("rightView", rightView)
	printLineInfo("mainView", mainView)

	fmt.Println("termW:", termW, "chatPaneWidth:", chatPaneWidth, "rightColumnWidth:", rightColumnWidth)
}

func printLineInfo(name, s string) {
	lines := strings.Split(s, "\n")
	fmt.Printf("== %s lines=%d totalWidth=%d\n", name, len(lines), lipgloss.Width(s))
	for i, line := range lines {
		fmt.Printf("%s line %d width=%d: %q\n", name, i, lipgloss.Width(line), line)
		if i > 10 {
			break
		}
	}
}

// Minimal GetThemeStyles copy for diag
func GetThemeStyles(ui *glitter.UI) ThemeStyles {
	return ThemeStyles{
		MainPane: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ui.Theme.Primary.Foreground)).BorderBackground(lipgloss.Color(ui.Theme.Primary.Background)).Background(lipgloss.Color(ui.Theme.Primary.Background)),
	}
}

// Minimal ThemeStyles to satisfy compile
type ThemeStyles struct {
	MainPane lipgloss.Style
}
