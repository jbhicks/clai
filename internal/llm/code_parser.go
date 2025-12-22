package llm

import (
	"clai/internal/logger"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type CodeBlock struct {
	Language string
	Code     string
}

func ParseCodeBlocks(content string) []CodeBlock {
	// Match complete code blocks
	codeTagRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)</code>`)
	matches := codeTagRegex.FindAllStringSubmatch(content, -1)

	var blocks []CodeBlock
	for _, match := range matches {
		if len(match) >= 3 {
			blocks = append(blocks, CodeBlock{
				Language: match[1],
				Code:     strings.TrimSpace(match[2]),
			})
		}
	}

	// Also match incomplete code blocks (missing closing tag or partial closing tag)
	// This handles cases like "</code" without the ">" or no closing tag at all
	incompleteRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)(?:</code|$)`)
	if len(blocks) == 0 {
		// Only look for incomplete blocks if we didn't find complete ones
		incompleteMatches := incompleteRegex.FindAllStringSubmatch(content, -1)

		for _, match := range incompleteMatches {
			if len(match) >= 3 {
				blocks = append(blocks, CodeBlock{
					Language: match[1],
					Code:     strings.TrimSpace(match[2]),
				})
			}
		}
	}

	return blocks
}

func StripCodeTags(content string) string {
	codeTagRegex := regexp.MustCompile(`(?s)<code\s+language="[^"]+">.*?</code>`)
	cleaned := codeTagRegex.ReplaceAllString(content, "")

	incompleteTagRegex := regexp.MustCompile(`(?s)<code\s+language="[^"]+">.*`)
	cleaned = incompleteTagRegex.ReplaceAllString(cleaned, "")

	return strings.TrimSpace(cleaned)
}

func StripTextBasedFunctionCalls(content string) string {
	functionCallRegex := regexp.MustCompile(`<function=[a-z_]+>?`)
	return functionCallRegex.ReplaceAllString(content, "")
}

func stripTrailingBackgroundPadding(line string) string {
	re := regexp.MustCompile(`\x1b\[48;[0-9;]+m\s+\x1b\[0m$`)
	return re.ReplaceAllString(line, "")
}

func RenderWithSyntaxHighlighting(content string, maxWidth int, codeBlockBadge, codeBlockContainer lipgloss.Style) string {
	blocks := ParseCodeBlocks(content)

	codeContainerWidth := maxWidth - codeBlockContainer.GetHorizontalFrameSize()

	renderCodeBlock := func(language, code string) string {
		markdown := fmt.Sprintf("```%s\n%s\n```", language, code)

		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(codeContainerWidth),
		)
		if err != nil {
			logger.Warn("[SYNTAX-HIGHLIGHT] Failed to create glamour renderer: %v", err)
			return markdown
		}

		rendered, err := renderer.Render(markdown)
		if err != nil {
			logger.Warn("[SYNTAX-HIGHLIGHT] Failed to render markdown: %v", err)
			return markdown
		}

		lines := strings.Split(rendered, "\n")
		var cleanLines []string
		for _, line := range lines {
			cleanedLine := stripTrailingBackgroundPadding(line)
			trimmed := strings.TrimRight(cleanedLine, " ")
			if trimmed != "" {
				cleanLines = append(cleanLines, trimmed)
			}
		}

		codeRendered := strings.Join(cleanLines, "\n")

		badge := codeBlockBadge.Render(fmt.Sprintf("🔧 %s", language))
		contentWithBadge := badge + "\n" + codeRendered

		return codeBlockContainer.Render(contentWithBadge)
	}

	result := content

	if len(blocks) > 0 {
		codeTagRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)</code>`)
		result = codeTagRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := codeTagRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}

			language := submatches[1]
			code := strings.TrimSpace(submatches[2])

			return renderCodeBlock(language, code)
		})

		incompleteRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)(?:</code|$)`)
		result = incompleteRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := incompleteRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}

			language := submatches[1]
			code := strings.TrimSpace(submatches[2])

			return renderCodeBlock(language, code)
		})
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		logger.Warn("[MARKDOWN-RENDER] Failed to create glamour renderer: %v", err)
		return result
	}

	rendered, err := renderer.Render(result)
	if err != nil {
		logger.Warn("[MARKDOWN-RENDER] Failed to render markdown: %v", err)
		return result
	}

	lines := strings.Split(rendered, "\n")
	var cleanLines []string
	for _, line := range lines {
		cleanedLine := stripTrailingBackgroundPadding(line)
		trimmed := strings.TrimRight(cleanedLine, " ")
		cleanLines = append(cleanLines, trimmed)
	}

	// Trim leading and trailing empty lines
	for len(cleanLines) > 0 && cleanLines[0] == "" {
		cleanLines = cleanLines[1:]
	}
	for len(cleanLines) > 0 && cleanLines[len(cleanLines)-1] == "" {
		cleanLines = cleanLines[:len(cleanLines)-1]
	}

	return strings.Join(cleanLines, "\n")
}
