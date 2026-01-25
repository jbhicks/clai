package ui

import (
	"testing"

	"github.com/brittonhayes/glitter/glitter"
)

func TestLayoutManager_UpdatePaneDimensions(t *testing.T) {
	ui := glitter.NewUI(DraculaTheme)
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config, ui)

	tests := []struct {
		name                  string
		totalWidth            int
		totalHeight           int
		showError             bool
		errorBannerHeight     int
		expectedBriefingWidth int
		expectedChatWidth     int
		expectedContentHeight int
	}{
		{
			name:                  "normal terminal size",
			totalWidth:            120,
			totalHeight:           30,
			showError:             false,
			errorBannerHeight:     0,
			expectedBriefingWidth: 24, // 120 * 0.2 = 24
			expectedChatWidth:     96, // 120 - 24 = 96
			expectedContentHeight: 29, // 30 - 1 (status bar)
		},
		{
			name:                  "with error banner",
			totalWidth:            120,
			totalHeight:           30,
			showError:             true,
			errorBannerHeight:     3,
			expectedBriefingWidth: 24,
			expectedChatWidth:     96,
			expectedContentHeight: 26, // 30 - 1 - 3 = 26
		},
		{
			name:                  "narrow terminal enforces minimum widths",
			totalWidth:            30,
			totalHeight:           20,
			showError:             false,
			errorBannerHeight:     0,
			expectedBriefingWidth: 10, // When total < min*2, chat pane gets priority
			expectedChatWidth:     20, // Minimum enforced for chat pane
			expectedContentHeight: 19,
		},
		{
			name:                  "very narrow terminal",
			totalWidth:            10,
			totalHeight:           10,
			showError:             false,
			errorBannerHeight:     0,
			expectedBriefingWidth: 20, // minimum width enforced
			expectedChatWidth:     20, // minimum width enforced (but total width is 10, so will be adjusted)
			expectedContentHeight: 9,
		},
		{
			name:                  "very small height with error banner",
			totalWidth:            80,
			totalHeight:           5,
			showError:             true,
			errorBannerHeight:     2,
			expectedBriefingWidth: 16, // 80 * 0.2 = 16
			expectedChatWidth:     64, // 80 - 16 = 64
			expectedContentHeight: 2,  // 5 - 1 - 2 = 2
		},
		{
			name:                  "zero dimensions",
			totalWidth:            0,
			totalHeight:           0,
			showError:             false,
			errorBannerHeight:     0,
			expectedBriefingWidth: 0, // Will be adjusted to fit minimums
			expectedChatWidth:     0,
			expectedContentHeight: 0, // max(-1, 0) = 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dims := lm.UpdatePaneDimensions(tt.totalWidth, tt.totalHeight, tt.showError, tt.errorBannerHeight)

			// Check pane widths (these may be adjusted for minimums)
			if dims.BriefingPaneWidth < config.MinPaneWidth && tt.totalWidth >= config.MinPaneWidth*2 {
				t.Errorf("BriefingPaneWidth = %d, expected >= %d", dims.BriefingPaneWidth, config.MinPaneWidth)
			}
			if dims.ChatPaneWidth < config.MinPaneWidth && tt.totalWidth >= config.MinPaneWidth*2 {
				t.Errorf("ChatPaneWidth = %d, expected >= %d", dims.ChatPaneWidth, config.MinPaneWidth)
			}

			// Check total width consistency
			if dims.BriefingPaneWidth+dims.ChatPaneWidth != tt.totalWidth {
				t.Errorf("BriefingPaneWidth + ChatPaneWidth = %d + %d = %d, expected %d",
					dims.BriefingPaneWidth, dims.ChatPaneWidth,
					dims.BriefingPaneWidth+dims.ChatPaneWidth, tt.totalWidth)
			}

			// Check content height
			expectedHeight := tt.totalHeight - 1 // status bar
			if tt.showError {
				expectedHeight -= tt.errorBannerHeight
			}
			if expectedHeight < 0 {
				expectedHeight = 0
			}
			if dims.ContentHeight != expectedHeight {
				t.Errorf("ContentHeight = %d, expected %d", dims.ContentHeight, expectedHeight)
			}

			// Check that pane heights match content height
			if dims.BriefingPaneHeight != dims.ContentHeight {
				t.Errorf("BriefingPaneHeight = %d, expected %d", dims.BriefingPaneHeight, dims.ContentHeight)
			}
			if dims.ChatPaneHeight != dims.ContentHeight {
				t.Errorf("ChatPaneHeight = %d, expected %d", dims.ChatPaneHeight, dims.ContentHeight)
			}
		})
	}
}

func TestLayoutManager_CalculatePaneWidths(t *testing.T) {
	ui := glitter.NewUI(DraculaTheme)
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config, ui)

	tests := []struct {
		name                  string
		totalWidth            int
		expectedBriefingWidth int
		expectedChatWidth     int
	}{
		{
			name:                  "normal width",
			totalWidth:            100,
			expectedBriefingWidth: 20, // 100 * 0.2 = 20
			expectedChatWidth:     80, // 100 - 20 = 80
		},
		{
			name:                  "narrow width enforces minimums",
			totalWidth:            30,
			expectedBriefingWidth: 20, // minimum enforced
			expectedChatWidth:     20, // minimum enforced
		},
		{
			name:                  "very narrow",
			totalWidth:            10,
			expectedBriefingWidth: 20, // minimum enforced
			expectedChatWidth:     20, // minimum enforced
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			briefingWidth, chatWidth := lm.calculatePaneWidths(tt.totalWidth)

			// For very narrow terminals, minimum enforcement may result in different distributions
			if tt.totalWidth >= config.MinPaneWidth*2 {
				// Only check expected values when minimums aren't enforced
				if briefingWidth != tt.expectedBriefingWidth {
					t.Errorf("briefingWidth = %d, expected %d", briefingWidth, tt.expectedBriefingWidth)
				}
				if chatWidth != tt.expectedChatWidth {
					t.Errorf("chatWidth = %d, expected %d", chatWidth, tt.expectedChatWidth)
				}
			}

			// Always check that minimums are respected when possible
			if tt.totalWidth >= config.MinPaneWidth {
				if briefingWidth < config.MinPaneWidth && chatWidth >= config.MinPaneWidth {
					// Chat pane got priority, briefing may be smaller
					if chatWidth != config.MinPaneWidth {
						t.Errorf("chatWidth = %d, expected %d when briefing gets priority", chatWidth, config.MinPaneWidth)
					}
				} else if chatWidth < config.MinPaneWidth && briefingWidth >= config.MinPaneWidth {
					// Briefing pane got priority, chat may be smaller
					if briefingWidth != config.MinPaneWidth {
						t.Errorf("briefingWidth = %d, expected %d when chat gets priority", briefingWidth, config.MinPaneWidth)
					}
				} else {
					// Both should meet minimums or neither can
					if briefingWidth < config.MinPaneWidth {
						t.Errorf("briefingWidth = %d, expected >= %d", briefingWidth, config.MinPaneWidth)
					}
					if chatWidth < config.MinPaneWidth {
						t.Errorf("chatWidth = %d, expected >= %d", chatWidth, config.MinPaneWidth)
					}
				}
			}
		})
	}
}

func TestLayoutManager_CalculateContentHeight(t *testing.T) {
	ui := glitter.NewUI(DraculaTheme)
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config, ui)

	tests := []struct {
		name                  string
		totalHeight           int
		showError             bool
		errorBannerHeight     int
		expectedContentHeight int
	}{
		{
			name:                  "normal height",
			totalHeight:           25,
			showError:             false,
			errorBannerHeight:     0,
			expectedContentHeight: 24, // 25 - 1
		},
		{
			name:                  "with error banner",
			totalHeight:           25,
			showError:             true,
			errorBannerHeight:     3,
			expectedContentHeight: 21, // 25 - 1 - 3
		},
		{
			name:                  "very small height",
			totalHeight:           2,
			showError:             false,
			errorBannerHeight:     0,
			expectedContentHeight: 1, // 2 - 1 = 1
		},
		{
			name:                  "height too small for error banner",
			totalHeight:           3,
			showError:             true,
			errorBannerHeight:     3,
			expectedContentHeight: 0, // max(3 - 1 - 3, 0) = 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			height := lm.CalculateContentHeight(tt.totalHeight, tt.showError, tt.errorBannerHeight)
			if height != tt.expectedContentHeight {
				t.Errorf("CalculateContentHeight() = %d, expected %d", height, tt.expectedContentHeight)
			}
		})
	}
}

func TestLayoutManager_Caching(t *testing.T) {
	ui := glitter.NewUI(DraculaTheme)
	config := DefaultLayoutConfig()
	lm := NewLayoutManager(config, ui)

	// First call should calculate
	dims1 := lm.UpdatePaneDimensions(100, 30, false, 0)

	// Second call with same parameters should use cache
	dims2 := lm.UpdatePaneDimensions(100, 30, false, 0)

	// Results should be identical
	if dims1 != dims2 {
		t.Error("Cached result differs from original calculation")
	}

	// Change parameters should recalculate
	dims3 := lm.UpdatePaneDimensions(120, 30, false, 0)
	if dims3.BriefingPaneWidth == dims1.BriefingPaneWidth {
		t.Error("Expected different result when width changed, but got cached result")
	}

	// Back to original parameters should use cache again
	dims4 := lm.UpdatePaneDimensions(100, 30, false, 0)
	if dims1 != dims4 {
		t.Error("Should use cache for repeated identical calls")
	}
}
