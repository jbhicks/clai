package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/brittonhayes/glitter/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Custom theme definitions with vibrant colors
var DraculaTheme = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("#282a36"),
		Foreground:    lipgloss.Color("#f8f8f2"),
		DimForeground: lipgloss.Color("#6272a4"),
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("#21222c"),
		Red:     lipgloss.Color("#ff5555"),
		Green:   lipgloss.Color("#50fa7b"),
		Yellow:  lipgloss.Color("#f1fa8c"),
		Blue:    lipgloss.Color("#bd93f9"),
		Magenta: lipgloss.Color("#ff79c6"),
		Cyan:    lipgloss.Color("#8be9fd"),
		White:   lipgloss.Color("#f8f8f2"),
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("#6272a4"),
		Red:     lipgloss.Color("#ff6e6e"),
		Green:   lipgloss.Color("#69ff94"),
		Yellow:  lipgloss.Color("#ffffa5"),
		Blue:    lipgloss.Color("#d6acff"),
		Magenta: lipgloss.Color("#ff92df"),
		Cyan:    lipgloss.Color("#a4ffff"),
		White:   lipgloss.Color("#ffffff"),
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("#191a21"),
		Red:     lipgloss.Color("#8b3434"),
		Green:   lipgloss.Color("#4d8a5d"),
		Yellow:  lipgloss.Color("#8a8654"),
		Blue:    lipgloss.Color("#7057a3"),
		Magenta: lipgloss.Color("#8b4970"),
		Cyan:    lipgloss.Color("#5b8a8a"),
		White:   lipgloss.Color("#8a8a8a"),
	},
}

var TokyoNightTheme = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("#1a1b26"),
		Foreground:    lipgloss.Color("#c0caf5"),
		DimForeground: lipgloss.Color("#565f89"),
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("#15161e"),
		Red:     lipgloss.Color("#f7768e"),
		Green:   lipgloss.Color("#9ece6a"),
		Yellow:  lipgloss.Color("#e0af68"),
		Blue:    lipgloss.Color("#7aa2f7"),
		Magenta: lipgloss.Color("#bb9af7"),
		Cyan:    lipgloss.Color("#7dcfff"),
		White:   lipgloss.Color("#c0caf5"),
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("#414868"),
		Red:     lipgloss.Color("#ff9db4"),
		Green:   lipgloss.Color("#b9f27c"),
		Yellow:  lipgloss.Color("#ffc777"),
		Blue:    lipgloss.Color("#82aaff"),
		Magenta: lipgloss.Color("#c099ff"),
		Cyan:    lipgloss.Color("#86e1fc"),
		White:   lipgloss.Color("#ffffff"),
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("#0f0f14"),
		Red:     lipgloss.Color("#8b4a57"),
		Green:   lipgloss.Color("#5f7a52"),
		Yellow:  lipgloss.Color("#8a6f4f"),
		Blue:    lipgloss.Color("#4f5f8a"),
		Magenta: lipgloss.Color("#6f5a8a"),
		Cyan:    lipgloss.Color("#4f7a8a"),
		White:   lipgloss.Color("#6f7a8a"),
	},
}

var CatppuccinTheme = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("#1e1e2e"),
		Foreground:    lipgloss.Color("#cdd6f4"),
		DimForeground: lipgloss.Color("#6c7086"),
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("#181825"),
		Red:     lipgloss.Color("#f38ba8"),
		Green:   lipgloss.Color("#a6e3a1"),
		Yellow:  lipgloss.Color("#f9e2af"),
		Blue:    lipgloss.Color("#89b4fa"),
		Magenta: lipgloss.Color("#cba6f7"),
		Cyan:    lipgloss.Color("#89dceb"),
		White:   lipgloss.Color("#cdd6f4"),
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("#585b70"),
		Red:     lipgloss.Color("#f5a9c1"),
		Green:   lipgloss.Color("#b8f0ba"),
		Yellow:  lipgloss.Color("#fdefc8"),
		Blue:    lipgloss.Color("#a5c9ff"),
		Magenta: lipgloss.Color("#ddc0ff"),
		Cyan:    lipgloss.Color("#a5ecf5"),
		White:   lipgloss.Color("#ffffff"),
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("#11111b"),
		Red:     lipgloss.Color("#8a5465"),
		Green:   lipgloss.Color("#698562"),
		Yellow:  lipgloss.Color("#8a7d62"),
		Blue:    lipgloss.Color("#556a8a"),
		Magenta: lipgloss.Color("#75628a"),
		Cyan:    lipgloss.Color("#557a8a"),
		White:   lipgloss.Color("#7d8a9d"),
	},
}

var SolarizedDarkTheme = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("#002b36"),
		Foreground:    lipgloss.Color("#839496"),
		DimForeground: lipgloss.Color("#586e75"),
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("#073642"),
		Red:     lipgloss.Color("#dc322f"),
		Green:   lipgloss.Color("#859900"),
		Yellow:  lipgloss.Color("#b58900"),
		Blue:    lipgloss.Color("#268bd2"),
		Magenta: lipgloss.Color("#d33682"),
		Cyan:    lipgloss.Color("#2aa198"),
		White:   lipgloss.Color("#eee8d5"),
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("#586e75"),
		Red:     lipgloss.Color("#f06d6d"),
		Green:   lipgloss.Color("#a3ca3b"),
		Yellow:  lipgloss.Color("#f4b73f"),
		Blue:    lipgloss.Color("#5eafef"),
		Magenta: lipgloss.Color("#ef5ebb"),
		Cyan:    lipgloss.Color("#4dd2c7"),
		White:   lipgloss.Color("#fdf6e3"),
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("#001f27"),
		Red:     lipgloss.Color("#7a1f1d"),
		Green:   lipgloss.Color("#4f5c00"),
		Yellow:  lipgloss.Color("#6a5000"),
		Blue:    lipgloss.Color("#1a5079"),
		Magenta: lipgloss.Color("#7a1f4f"),
		Cyan:    lipgloss.Color("#1a5d57"),
		White:   lipgloss.Color("#83857f"),
	},
}

// 256-color compatible themes for remote terminals
var DraculaTheme256 = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("235"), // Darker, richer background
		Foreground:    lipgloss.Color("255"), // Pure white
		DimForeground: lipgloss.Color("60"),  // Better dim blue
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("232"), // Very dark gray
		Red:     lipgloss.Color("197"), // Brighter red
		Green:   lipgloss.Color("48"),  // More vibrant green
		Yellow:  lipgloss.Color("220"), // Richer yellow
		Blue:    lipgloss.Color("99"),  // Deeper purple-blue
		Magenta: lipgloss.Color("205"), // Brighter pink
		Cyan:    lipgloss.Color("51"),  // Keep cyan as is
		White:   lipgloss.Color("255"), // Pure white
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("60"),  // Better dim blue
		Red:     lipgloss.Color("203"), // Keep as is
		Green:   lipgloss.Color("77"),  // Keep as is
		Yellow:  lipgloss.Color("229"), // Keep as is
		Blue:    lipgloss.Color("147"), // Keep as is
		Magenta: lipgloss.Color("206"), // Keep as is
		Cyan:    lipgloss.Color("159"), // Keep as is
		White:   lipgloss.Color("15"),  // Keep as is
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("233"), // Very dark
		Red:     lipgloss.Color("88"),  // Keep as is
		Green:   lipgloss.Color("28"),  // Keep as is
		Yellow:  lipgloss.Color("58"),  // Keep as is
		Blue:    lipgloss.Color("61"),  // Keep as is
		Magenta: lipgloss.Color("89"),  // Keep as is
		Cyan:    lipgloss.Color("30"),  // Keep as is
		White:   lipgloss.Color("102"), // Keep as is
	},
}

var TokyoNightTheme256 = theme.Theme{
	Primary: theme.Primary{
		Background:    lipgloss.Color("233"), // Richer dark background
		Foreground:    lipgloss.Color("110"), // Keep as is
		DimForeground: lipgloss.Color("59"),  // Keep as is
	},
	Normal: theme.Normal{
		Black:   lipgloss.Color("232"), // Very dark gray
		Red:     lipgloss.Color("167"), // Better red
		Green:   lipgloss.Color("72"),  // Keep as is
		Yellow:  lipgloss.Color("179"), // Keep as is
		Blue:    lipgloss.Color("111"), // Keep as is
		Magenta: lipgloss.Color("140"), // Keep as is
		Cyan:    lipgloss.Color("75"),  // Keep as is
		White:   lipgloss.Color("110"), // Keep as is
	},
	Bright: theme.Bright{
		Black:   lipgloss.Color("59"),  // Keep as is
		Red:     lipgloss.Color("204"), // Keep as is
		Green:   lipgloss.Color("77"),  // Keep as is
		Yellow:  lipgloss.Color("222"), // Keep as is
		Blue:    lipgloss.Color("111"), // Keep as is
		Magenta: lipgloss.Color("147"), // Keep as is
		Cyan:    lipgloss.Color("87"),  // Keep as is
		White:   lipgloss.Color("15"),  // Keep as is
	},
	Dim: theme.Dim{
		Black:   lipgloss.Color("234"), // Better dim black
		Red:     lipgloss.Color("96"),  // Keep as is
		Green:   lipgloss.Color("64"),  // Keep as is
		Yellow:  lipgloss.Color("100"), // Keep as is
		Blue:    lipgloss.Color("61"),  // Keep as is
		Magenta: lipgloss.Color("62"),  // Keep as is
		Cyan:    lipgloss.Color("66"),  // Keep as is
		White:   lipgloss.Color("59"),  // Keep as is
	},
}

// Theme represents a complete theme configuration
type Theme struct {
	Name         string      // Human-readable name
	GlitterUI    *glitter.UI // Glitter UI instance
	GlamourStyle string      // Glamour style name for markdown rendering
	Is256Color   bool        // Whether this is a 256-color theme
}

// ThemeRegistry is the central registry of all available themes
var ThemeRegistry = []Theme{
	{
		Name:         "Dracula",
		GlitterUI:    glitter.NewUI(DraculaTheme),
		GlamourStyle: "dark",
		Is256Color:   false,
	},
	{
		Name:         "Tokyo Night",
		GlitterUI:    glitter.NewUI(TokyoNightTheme),
		GlamourStyle: "dark",
		Is256Color:   false,
	},
	{
		Name:         "Catppuccin",
		GlitterUI:    glitter.NewUI(CatppuccinTheme),
		GlamourStyle: "dark",
		Is256Color:   false,
	},
	{
		Name:         "Solarized Dark",
		GlitterUI:    glitter.NewUI(SolarizedDarkTheme),
		GlamourStyle: "dark",
		Is256Color:   false,
	},
	{
		Name:         "Dracula 256",
		GlitterUI:    glitter.NewUI(DraculaTheme256),
		GlamourStyle: "dark",
		Is256Color:   true,
	},
	{
		Name:         "Tokyo Night 256",
		GlitterUI:    glitter.NewUI(TokyoNightTheme256),
		GlamourStyle: "dark",
		Is256Color:   true,
	},
}

// HasTrueColor checks if terminal supports true color (16M colors)
func HasTrueColor() bool {
	// Check for forced true color mode (for users who know their terminal supports it)
	if os.Getenv("CLAI_FORCE_TRUE_COLOR") == "1" {
		return true
	}

	// Check COLORTERM environment variable
	colorterm := os.Getenv("COLORTERM")
	if colorterm == "truecolor" || colorterm == "24bit" {
		return true
	}

	// Check for WezTerm specifically
	term := os.Getenv("TERM_PROGRAM")
	if term == "wezterm" {
		return true
	}

	// Check for common true color terminals
	term = os.Getenv("TERM")
	if strings.Contains(term, "xterm-256color") ||
		strings.Contains(term, "screen-256color") ||
		strings.Contains(term, "tmux-256color") {
		// These TERM values often support true color, but we need to be more careful
		// in remote terminal scenarios. Let's check tput colors as a fallback.
		cmd := exec.Command("tput", "colors")
		cmd.Stderr = nil
		if output, err := cmd.Output(); err == nil {
			if colors := strings.TrimSpace(string(output)); colors != "" {
				if num, err := strconv.Atoi(colors); err == nil && num > 256 {
					return true
				}
			}
		}
		// If tput colors <= 256, don't assume true color support
		return false
	}

	// Check if tput reports more than 256 colors
	cmd := exec.Command("tput", "colors")
	cmd.Stderr = nil // Suppress stderr
	if output, err := cmd.Output(); err == nil {
		if colors := strings.TrimSpace(string(output)); colors != "" {
			if num, err := strconv.Atoi(colors); err == nil && num > 256 {
				return true
			}
		}
	}

	return false
}

// GetAvailableThemes returns the appropriate theme set based on terminal color support
func GetAvailableThemes() []*glitter.UI {
	hasTrueColor := HasTrueColor()
	var themes []*glitter.UI

	for _, theme := range ThemeRegistry {
		if hasTrueColor && !theme.Is256Color {
			themes = append(themes, theme.GlitterUI)
		} else if !hasTrueColor && theme.Is256Color {
			themes = append(themes, theme.GlitterUI)
		}
	}

	return themes
}

// GetAvailableThemeNames returns the appropriate theme name set based on terminal color support
func GetAvailableThemeNames() []string {
	hasTrueColor := HasTrueColor()
	var names []string

	for _, theme := range ThemeRegistry {
		if hasTrueColor && !theme.Is256Color {
			names = append(names, theme.Name)
		} else if !hasTrueColor && theme.Is256Color {
			names = append(names, theme.Name)
		}
	}

	return names
}

// GetThemeByName finds a theme by name with validation
func GetThemeByName(name string) (*Theme, error) {
	for _, theme := range ThemeRegistry {
		if theme.Name == name {
			return &theme, nil
		}
	}
	return nil, fmt.Errorf("theme '%s' not found", name)
}

// ListThemes returns all available theme names
func ListThemes() []string {
	var names []string
	for _, theme := range ThemeRegistry {
		names = append(names, theme.Name)
	}
	return names
}

// GetThemeGlamourStyle returns the Glamour style name for a given theme name
func GetThemeGlamourStyle(themeName string) (string, error) {
	theme, err := GetThemeByName(themeName)
	if err != nil {
		return "", err
	}
	return theme.GlamourStyle, nil
}

// GetThemeStyles returns lipgloss styles compatible with the current glitter theme
func GetThemeStyles(ui *glitter.UI) ThemeStyles {
	// Explicitly set color profile based on our detection
	if HasTrueColor() {
		lipgloss.SetColorProfile(termenv.TrueColor)
	} else {
		lipgloss.SetColorProfile(termenv.ANSI256)
	}

	// Base style for all message bubbles - common properties shared across message types
	baseMessageStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	return ThemeStyles{
		MainPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ui.Theme.Primary.Foreground)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Padding(1, 2),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Primary.Foreground)).
			Bold(true),

		UserMessage: baseMessageStyle.
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Blue)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Bold(true),

		AssistantMessage: baseMessageStyle.
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Green)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Primary.Foreground)),

		ToolMessage: baseMessageStyle.
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Blue)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Dim.White)).
			Italic(true),

		ToolBadge: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Bright.Blue)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Padding(0, 2),

		CodeBlockBadge: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Bright.Magenta)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Padding(0, 1).
			Bold(true),

		CodeBlockContainer: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Magenta)).
			Background(lipgloss.Color(ui.Theme.Dim.Black)).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1),

		ScrollIndicator: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Bright.Cyan)).
			Foreground(lipgloss.Color(ui.Theme.Primary.Background)).
			Padding(0, 1).
			Align(lipgloss.Center).
			Bold(true),

		InputFocused: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Yellow)).
			Padding(0, 1),

		InputUnfocused: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		Tooltip: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Bright.Blue)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Padding(0, 1),

		BackgroundWrapper: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Align(lipgloss.Left),

		ActionCompleted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ui.Theme.Bright.Green)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		ActionFailed: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ui.Theme.Bright.Red)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		ActionInProgress: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ui.Theme.Bright.Yellow)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		HelpBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Align(lipgloss.Center),

		LogDim: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ui.Theme.Primary.DimForeground)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		LogValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ui.Theme.Primary.Foreground)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),
	}
}

// ThemeStyles contains all the lipgloss styles for a theme
type ThemeStyles struct {
	MainPane           lipgloss.Style
	StatusBar          lipgloss.Style
	UserMessage        lipgloss.Style
	AssistantMessage   lipgloss.Style
	ToolMessage        lipgloss.Style
	ToolBadge          lipgloss.Style
	CodeBlockBadge     lipgloss.Style
	CodeBlockContainer lipgloss.Style
	ScrollIndicator    lipgloss.Style
	InputFocused       lipgloss.Style
	InputUnfocused     lipgloss.Style
	Tooltip            lipgloss.Style
	BackgroundWrapper  lipgloss.Style
	ActionCompleted    lipgloss.Style
	ActionFailed       lipgloss.Style
	ActionInProgress   lipgloss.Style
	HelpBox            lipgloss.Style
	LogDim             lipgloss.Style
	LogValue           lipgloss.Style
}

// test comment
// test
