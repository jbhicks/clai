package ui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/brittonhayes/glitter/glitter"
)

// Pre-compiled regex patterns for performance (ansiRegex is defined in chat.go)
var (
	ansiOrCharRegex = regexp.MustCompile(`(\x1b\[[0-9;]*m)|([^\x1b])`)
)

// AnsiSequence represents a parsed ANSI escape sequence
type AnsiSequence struct {
	Type       string // "foreground", "background", "reset"
	ColorType  string // "normal", "bright", "dim", "rgb"
	ColorValue string // hex code or ANSI number
	Raw        string // original escape sequence
	Position   int    // position in the line
}

// LineAnalysis contains detailed analysis of a single line
type LineAnalysis struct {
	LineNumber        int                 `json:"lineNumber"`
	Content           string              `json:"content"`
	Length            int                 `json:"length"`
	BackgroundColor   string              `json:"backgroundColor"`
	HasBackground     bool                `json:"hasBackground"`
	CoveragePercent   float64             `json:"coveragePercent"`
	TransparencyGaps  []TransparencyGap   `json:"transparencyGaps,omitempty"`
	CharacterAnalysis []CharacterAnalysis `json:"characterAnalysis,omitempty"`
}

// TransparencyGap represents a region without background color
type TransparencyGap struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Length      int    `json:"length"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// CharacterAnalysis represents analysis of a single character position
type CharacterAnalysis struct {
	Position      int    `json:"position"`
	Char          string `json:"char"`
	HasBackground bool   `json:"hasBackground"`
	BackgroundRGB string `json:"backgroundRgb,omitempty"`
	ForegroundRGB string `json:"foregroundRgb,omitempty"`
	Status        string `json:"status"` // "ok", "missing_bg", "wrong_bg"
	ExpectedBG    string `json:"expectedBg,omitempty"`
}

// ComponentAnalysis contains analysis for a UI component
type ComponentAnalysis struct {
	Name              string         `json:"name"`
	BackgroundColor   string         `json:"backgroundColor"`
	ExpectedColor     string         `json:"expectedColor"`
	ColorMatch        bool           `json:"colorMatch"`
	TotalLines        int            `json:"totalLines"`
	TotalCoverage     float64        `json:"totalCoverage"`
	TransparencyCount int            `json:"transparencyCount"`
	LinesWithIssues   int            `json:"linesWithIssues"`
	LineAnalysis      []LineAnalysis `json:"lineAnalysis,omitempty"`
}

// FullRenderAnalysis contains the complete analysis of the UI render
type FullRenderAnalysis struct {
	Components            []ComponentAnalysis `json:"components"`
	OverallCoverage       float64             `json:"overallCoverage"`
	TotalTransparencyGaps int                 `json:"totalTransparencyGaps"`
	Summary               string              `json:"summary"`
}

// RGB to hex conversion helpers
var (
	ansiToRGB = map[string]string{
		// Normal colors
		"30": "#21222c", "31": "#ff5555", "32": "#50fa7b", "33": "#f1fa8c",
		"34": "#bd93f9", "35": "#ff79c6", "36": "#8be9fd", "37": "#f8f8f2",
		// Bright colors
		"90": "#6272a4", "91": "#ff6e6e", "92": "#69ff94", "93": "#ffffa5",
		"94": "#d6acff", "95": "#ff92df", "96": "#a4ffff", "97": "#ffffff",
		// Dim colors
		"2": "#6272a4",
	}
)

// Get color from ANSI sequence
func getColorFromSequence(seq AnsiSequence, isBackground bool) string {
	code := strings.TrimPrefix(seq.Raw, "\x1b[")
	code = strings.TrimSuffix(code, "m")

	parts := strings.Split(code, ";")
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			if num == 38 || num == 48 {
				continue // Skip truecolor for now
			}
			effectiveNum := num
			if isBackground && num >= 30 && num <= 37 {
				effectiveNum = num + 10
			} else if !isBackground && num >= 40 && num <= 47 {
				effectiveNum = num - 10
			}
			if rgb, ok := ansiToRGB[strconv.Itoa(effectiveNum)]; ok {
				return rgb
			}
		}
	}
	return ""
}

// Analyze background coverage for a single line - optimized single pass
func analyzeLineBackground(line string, expectedBG string) LineAnalysis {
	analysis := LineAnalysis{
		Content: ansiRegex.ReplaceAllString(line, ""),
		Length:  len(line),
	}

	hasCurrentBG := false
	currentBG := ""
	bgPositions := make(map[int]bool)
	var gaps []TransparencyGap

	matches := ansiOrCharRegex.FindAllStringSubmatchIndex(line, -1)
	pos := 0
	lastResetPos := -1

	for _, match := range matches {
		if len(match) >= 4 {
			if match[2] != -1 {
				// ANSI sequence
				seq := line[match[2]:match[3]]
				if strings.HasSuffix(seq, "m") {
					if seq == "\x1b[0m" || seq == "\x1b[m" {
						hasCurrentBG = false
						currentBG = ""
						lastResetPos = match[2]
					} else if strings.Contains(seq, "48") {
						hasCurrentBG = true
						currentBG = getColorFromSequence(AnsiSequence{Raw: seq}, true)
					}
				}
			} else if match[4] != -1 {
				// Regular character
				char := line[match[4]:match[5]]
				if hasCurrentBG {
					bgPositions[pos] = true
				} else if char == " " {
					// Space without background is a transparency gap
					gap := TransparencyGap{
						Start:   pos,
						End:     pos,
						Content: " ",
					}
					if lastResetPos > strings.LastIndex(line[:match[4]], "\x1b[0m") {
						gap.Description = "Reset without reapplying background"
					}
					gaps = append(gaps, gap)
				}
				pos++
			}
		}
	}

	analysis.HasBackground = len(bgPositions) > 0
	if len(bgPositions) > 0 {
		analysis.BackgroundColor = currentBG
		if pos > 0 {
			analysis.CoveragePercent = float64(len(bgPositions)) / float64(pos) * 100
		}
	}
	analysis.TransparencyGaps = gaps

	return analysis
}

// Analyze component for background coverage
func analyzeComponent(name string, content string, expectedBG string) ComponentAnalysis {
	lines := strings.Split(content, "\n")
	var lineAnalyses []LineAnalysis
	totalCoverage := 0.0
	linesWithIssues := 0

	for i, line := range lines {
		analysis := analyzeLineBackground(line, expectedBG)
		analysis.LineNumber = i
		lineAnalyses = append(lineAnalyses, analysis)

		if !analysis.HasBackground || analysis.CoveragePercent < 100 {
			linesWithIssues++
		}
		totalCoverage += analysis.CoveragePercent
	}

	avgCoverage := 0.0
	if len(lineAnalyses) > 0 {
		avgCoverage = totalCoverage / float64(len(lineAnalyses))
	}

	totalGaps := 0
	for _, la := range lineAnalyses {
		totalGaps += len(la.TransparencyGaps)
	}

	// Get actual background color from first line with content
	actualBG := ""
	for _, la := range lineAnalyses {
		if la.HasBackground && la.BackgroundColor != "" {
			actualBG = la.BackgroundColor
			break
		}
	}

	colorMatch := actualBG == expectedBG || actualBG == ""
	if actualBG == "" {
		actualBG = "(none detected)"
	}

	return ComponentAnalysis{
		Name:              name,
		BackgroundColor:   actualBG,
		ExpectedColor:     expectedBG,
		ColorMatch:        colorMatch,
		TotalLines:        len(lineAnalyses),
		TotalCoverage:     avgCoverage,
		TransparencyCount: totalGaps,
		LinesWithIssues:   linesWithIssues,
		LineAnalysis:      lineAnalyses,
	}
}

// AnalyzeFullRender performs complete analysis of the UI render
func AnalyzeFullRender(theme *glitter.UI, textarea, viewport, status, logPane string) FullRenderAnalysis {
	expectedBG := string(theme.Theme.Primary.Background)

	components := []ComponentAnalysis{
		analyzeComponent("textarea", textarea, expectedBG),
		analyzeComponent("viewport", viewport, expectedBG),
		analyzeComponent("status", status, expectedBG),
		analyzeComponent("log", logPane, expectedBG),
	}

	totalCoverage := 0.0
	totalGaps := 0
	for _, c := range components {
		totalCoverage += c.TotalCoverage
		totalGaps += c.TransparencyCount
	}
	avgCoverage := totalCoverage / float64(len(components))

	summary := "Analysis complete"
	if totalGaps > 0 {
		summary = "Found " + strconv.Itoa(totalGaps) + " transparency gap(s) across " +
			strconv.Itoa(len(components)) + " components"
	}

	return FullRenderAnalysis{
		Components:            components,
		OverallCoverage:       avgCoverage,
		TotalTransparencyGaps: totalGaps,
		Summary:               summary,
	}
}

// GenerateVisualBackgroundMap creates a visual representation of background coverage
func GenerateVisualBackgroundMap(line string) string {
	matches := ansiOrCharRegex.FindAllStringSubmatchIndex(line, -1)

	hasBG := false
	result := ""
	pos := 0

	for _, match := range matches {
		if len(match) >= 4 {
			if match[2] != -1 {
				seq := line[match[2]:match[3]]
				if seq == "\x1b[0m" || seq == "\x1b[m" {
					hasBG = false
				} else if strings.Contains(seq, "48") {
					hasBG = true
				}
			} else if match[4] != -1 {
				char := string(line[match[4]:match[5]])
				if char == " " {
					if hasBG {
						result += "█"
					} else {
						result += "░"
					}
				} else {
					if hasBG {
						result += "█"
					} else {
						result += char
					}
				}
				pos++
			}
		}
	}

	return result
}

// GetTransparencyReport generates a detailed transparency report
func GetTransparencyReport(analysis LineAnalysis) string {
	if len(analysis.TransparencyGaps) == 0 {
		return "No transparency issues detected"
	}

	report := "Transparency Issues:\n"
	for i, gap := range analysis.TransparencyGaps {
		report += strconv.Itoa(i+1) + ". Position " + strconv.Itoa(gap.Start)
		if gap.Description != "" {
			report += " (" + gap.Description + ")"
		}
		report += ": \"" + gap.Content + "\"\n"
	}
	return report
}
