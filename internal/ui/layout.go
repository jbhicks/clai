package ui

import (
	"strings"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/lipgloss"
)

// PaneDimensions holds the calculated dimensions for all panes
// All heights are the same (content height), widths follow the configured ratio with minimums enforced
type PaneDimensions struct {
	BriefingPaneWidth  int
	ChatPaneWidth      int
	BriefingPaneHeight int
	ChatPaneHeight     int
	ContentHeight      int
}

// LayoutManager centralizes all layout logic for the TUI
type LayoutManager struct {
	Config LayoutConfig
	Theme  *glitter.UI

	// Cached dimensions to avoid recalculation
	cachedPaneDims    *PaneDimensions
	cachedWidth       int
	cachedHeight      int
	cachedShowError   bool
	cachedErrorHeight int
}

// NewLayoutManager creates a layout manager with the given config and theme
func NewLayoutManager(config LayoutConfig, theme *glitter.UI) *LayoutManager {
	return &LayoutManager{
		Config: config,
		Theme:  theme,
	}
}

// MainLayout assembles the complete UI from components
// This is the primary layout method that handles pane modes and final composition
func (lm *LayoutManager) MainLayout(
	briefingView, chatView, logView string,
	statusBar, errorMsg string,
	width, height int,
	activePane ActivePane,
	showError bool,
) string {
	// Calculate pane dimensions using centralized logic
	briefingWidth, chatWidth := lm.calculatePaneWidths(width)

	// Get theme styles
	styles := GetThemeStyles(lm.Theme)

	// Apply pane styling and handle different pane modes
	var mainView string
	if activePane == LogPane {
		mainView = lm.logViewLayout(logView, width, height, styles)
	} else {
		briefingPane := lm.stylePane(briefingView, briefingWidth, height, activePane == BriefingRoom, styles)
		chatPane := lm.stylePane(chatView, chatWidth, height, activePane == ChatPane, styles)
		mainView = lm.mainViewLayout(briefingPane, chatPane)
	}

	// Add status bar and error handling
	return lm.finalLayout(mainView, statusBar, errorMsg, width, showError, styles)
}

// calculatePaneWidths returns (briefingWidth, chatWidth) using the layout config
// Applies the configured width ratio and enforces minimum pane widths
func (lm *LayoutManager) calculatePaneWidths(totalWidth int) (int, int) {
	return lm.calculatePaneWidthsWithRatio(totalWidth, lm.Config.ChatPaneWidthRatio)
}

// calculatePaneWidthsWithRatio is the core logic for calculating pane widths with minimum enforcement
func (lm *LayoutManager) calculatePaneWidthsWithRatio(totalWidth int, chatRatio float64) (briefingWidth, chatWidth int) {
	briefingWidth = int(float64(totalWidth) * (1.0 - chatRatio))
	chatWidth = totalWidth - briefingWidth

	// Enforce minimum widths
	if briefingWidth < lm.Config.MinPaneWidth {
		briefingWidth = lm.Config.MinPaneWidth
		chatWidth = totalWidth - briefingWidth
	}
	if chatWidth < lm.Config.MinPaneWidth {
		chatWidth = lm.Config.MinPaneWidth
		briefingWidth = totalWidth - chatWidth
	}

	return briefingWidth, chatWidth
}

// stylePane applies consistent styling to a pane with optional active highlighting
// active=true adds a yellow border to indicate the currently focused pane
func (lm *LayoutManager) stylePane(content string, width, height int, active bool, styles ThemeStyles) string {
	style := styles.MainPane.Width(width).Height(height)

	// ALWAYS set border background to prevent transparent gaps
	style = style.Copy().
		BorderForeground(lipgloss.Color(lm.Theme.Theme.Bright.Yellow)).
		BorderBackground(lipgloss.Color(lm.Theme.Theme.Primary.Background))

	if active {
		style = style.BorderForeground(lipgloss.Color(lm.Theme.Theme.Bright.Yellow))
	}

	return style.Render(content)
}

// mainViewLayout joins briefing and chat panes horizontally
// Returns a side-by-side layout of the two main panes
func (lm *LayoutManager) mainViewLayout(briefingPane, chatPane string) string {
	// Verify rendered widths don't exceed available space
	briefingWidth := lipgloss.Width(briefingPane)
	chatWidth := lipgloss.Width(chatPane)
	totalWidth := briefingWidth + chatWidth

	// If combined width exceeds available space, recalculate with constraints
	if totalWidth > lm.cachedWidth && lm.cachedWidth > 0 {
		// Recalculate panes with proper width constraints
		availableWidth := lm.cachedWidth
		newBriefingWidth, newChatWidth := lm.calculatePaneWidthsWithRatio(availableWidth, lm.Config.ChatPaneWidthRatio)

		// Re-render with corrected dimensions
		styles := GetThemeStyles(lm.Theme)
		briefingPane = lm.stylePane(briefingPane, newBriefingWidth, lm.cachedHeight, false, styles)
		chatPane = lm.stylePane(chatPane, newChatWidth, lm.cachedHeight, false, styles)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, briefingPane, chatPane)
}

// logViewLayout creates the full-screen log viewer layout
// Fills the entire terminal with log content, styled as an active pane
func (lm *LayoutManager) logViewLayout(logView string, width, height int, styles ThemeStyles) string {
	// Process log content with background filling
	logContent := lm.processLogContent(logView, width, height, styles)

	// Apply active styling to log pane
	logPaneStyle := styles.MainPane.
		Copy().
		BorderForeground(lipgloss.Color(lm.Theme.Theme.Bright.Yellow)).
		BorderBackground(lipgloss.Color(lm.Theme.Theme.Primary.Background)).
		Width(width).
		Height(height)

	return logPaneStyle.Render(logContent)
}

// processLogContent fills log content to full height with proper background
// Ensures consistent background colors and fills empty space to prevent artifacts
func (lm *LayoutManager) processLogContent(logContent string, width, height int, styles ThemeStyles) string {
	lines := strings.Split(logContent, "\n")
	var renderedLines []string

	// Render each line with consistent background
	for _, line := range lines {
		renderedLines = append(renderedLines, styles.BackgroundWrapper.Width(width).Render(line))
	}

	// Fill remaining height (accounting for status bar)
	targetHeight := height - 1 // -1 for status bar
	if targetHeight < 0 {
		targetHeight = 0
	}
	for len(renderedLines) < targetHeight {
		renderedLines = append(renderedLines, styles.BackgroundWrapper.Width(width).Render(""))
	}

	return strings.Join(renderedLines, "\n")
}

// finalLayout adds status bar and error banner to the main view
// Returns the complete terminal layout with status bar at bottom and optional error banner
func (lm *LayoutManager) finalLayout(mainView, statusBar, errorMsg string, width int, showError bool, styles ThemeStyles) string {
	// Add status bar
	statusBarRendered := styles.StatusBar.Width(width).Render(statusBar)
	layout := lipgloss.JoinVertical(lipgloss.Left, mainView, statusBarRendered)

	// Add error banner if needed
	if showError && errorMsg != "" {
		// Note: ErrorBanner style should be passed or accessed differently
		// For now, assume it's available in styles or passed separately
		errorRendered := lipgloss.NewStyle().Width(width).Render(errorMsg)
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, errorRendered)
	}

	return layout
}

// CalculateContentHeight returns available height for content areas
// Subtracts status bar height and optional error banner height from total height
func (lm *LayoutManager) CalculateContentHeight(totalHeight int, showError bool, errorBannerHeight int) int {
	height := totalHeight - StatusBarHeight
	if showError {
		height -= errorBannerHeight
	}
	return max(height, 0)
}

// UpdatePaneDimensions calculates and returns all pane dimensions based on terminal size and layout config
// Uses caching to avoid recalculation when inputs haven't changed
func (lm *LayoutManager) UpdatePaneDimensions(totalWidth, totalHeight int, showError bool, errorBannerHeight int) PaneDimensions {
	// Check cache first
	if lm.cachedPaneDims != nil &&
		lm.cachedWidth == totalWidth &&
		lm.cachedHeight == totalHeight &&
		lm.cachedShowError == showError &&
		lm.cachedErrorHeight == errorBannerHeight {
		return *lm.cachedPaneDims
	}

	// Calculate total height available for content (excluding status bar and potential error banner)
	contentHeight := totalHeight - 1 // Status bar always takes 1 row
	if showError {
		contentHeight -= errorBannerHeight
	}
	contentHeight = max(contentHeight, 0) // Ensure non-negative

	// Calculate pane widths using layout config ratio
	// lipgloss.Width() ensures rendered panes are exactly the specified width,
	// accounting for borders internally, so pane widths should sum to totalWidth
	briefingPaneWidth, chatPaneWidth := lm.calculatePaneWidthsWithRatio(totalWidth, lm.Config.ChatPaneWidthRatio)

	dims := PaneDimensions{
		BriefingPaneWidth:  briefingPaneWidth,
		ChatPaneWidth:      chatPaneWidth,
		BriefingPaneHeight: contentHeight, // Both panes use full content height
		ChatPaneHeight:     contentHeight,
		ContentHeight:      contentHeight,
	}

	// Cache the result
	lm.cachedPaneDims = &dims
	lm.cachedWidth = totalWidth
	lm.cachedHeight = totalHeight
	lm.cachedShowError = showError
	lm.cachedErrorHeight = errorBannerHeight

	return dims
}

// HelpLayout creates the overlay help dialog
// Centers a help box with the specified width ratio and minimum size constraints
func (lm *LayoutManager) HelpLayout(helpContent string, width, height int, styles ThemeStyles) string {
	helpWidth := int(float64(width) * lm.Config.HelpPaneWidthRatio)
	helpBox := styles.HelpBox.Width(max(helpWidth/2, 10)).Render(helpContent)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBox)
}
