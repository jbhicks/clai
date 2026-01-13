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
	// Parse all code block formats and collect them with their positions
	// This allows mixed formats in the same content

	type blockWithPosition struct {
		block CodeBlock
		pos   int
	}

	var blocksWithPos []blockWithPosition

	// Pattern 1: Full XML format (complete blocks)
	fullXMLRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)</code>`)
	for _, match := range fullXMLRegex.FindAllStringSubmatchIndex(content, -1) {
		lang := content[match[2]:match[3]]
		code := content[match[4]:match[5]]
		blocksWithPos = append(blocksWithPos, blockWithPosition{
			block: CodeBlock{
				Language: lang,
				Code:     strings.TrimSpace(code),
			},
			pos: match[0],
		})
	}

	// Pattern 2: Simplified XML format (complete blocks) - <code python>
	simplifiedXMLRegex := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)>(.+?)</code>`)
	for _, match := range simplifiedXMLRegex.FindAllStringSubmatchIndex(content, -1) {
		lang := content[match[2]:match[3]]
		code := content[match[4]:match[5]]

		// Normalize language names
		if lang == "js" {
			lang = "javascript"
		} else if lang == "py" {
			lang = "python"
		} else if lang == "sh" {
			lang = "bash"
		}

		blocksWithPos = append(blocksWithPos, blockWithPosition{
			block: CodeBlock{
				Language: lang,
				Code:     strings.TrimSpace(code),
			},
			pos: match[0],
		})
	}

	// Pattern 3: Markdown code blocks - ```python
	markdownRegex := regexp.MustCompile("(?s)```(bash|python|javascript|js|sh|py|go)\\s*\\n(.+?)```")
	for _, match := range markdownRegex.FindAllStringSubmatchIndex(content, -1) {
		lang := content[match[2]:match[3]]
		code := content[match[4]:match[5]]

		// Normalize language names
		if lang == "js" {
			lang = "javascript"
		} else if lang == "py" {
			lang = "python"
		} else if lang == "sh" {
			lang = "bash"
		}

		blocksWithPos = append(blocksWithPos, blockWithPosition{
			block: CodeBlock{
				Language: lang,
				Code:     strings.TrimSpace(code),
			},
			pos: match[0],
		})
	}

	// Handle incomplete blocks only if no complete blocks found
	if len(blocksWithPos) == 0 {
		// Incomplete full XML: <code language="python">... (no closing tag)
		incompleteFullXMLRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)(?:</code|$)`)
		for _, match := range incompleteFullXMLRegex.FindAllStringSubmatchIndex(content, -1) {
			lang := content[match[2]:match[3]]
			code := content[match[4]:match[5]]
			blocksWithPos = append(blocksWithPos, blockWithPosition{
				block: CodeBlock{
					Language: lang,
					Code:     strings.TrimSpace(code),
				},
				pos: match[0],
			})
		}

		// Incomplete simplified XML: <code python>... (no closing tag)
		if len(blocksWithPos) == 0 {
			incompleteSimplifiedRegex := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)>(.+?)(?:</code|$)`)
			for _, match := range incompleteSimplifiedRegex.FindAllStringSubmatchIndex(content, -1) {
				lang := content[match[2]:match[3]]
				code := content[match[4]:match[5]]

				if lang == "js" {
					lang = "javascript"
				} else if lang == "py" {
					lang = "python"
				} else if lang == "sh" {
					lang = "bash"
				}

				blocksWithPos = append(blocksWithPos, blockWithPosition{
					block: CodeBlock{
						Language: lang,
						Code:     strings.TrimSpace(code),
					},
					pos: match[0],
				})
			}
		}

		// Malformed simplified XML: <code python (missing > on opening tag, no closing tag)
		// This handles cases where streaming was cut off or the model output is malformed
		if len(blocksWithPos) == 0 {
			malformedSimplifiedRegex := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)\s+(.+?)(?:</code|$)`)
			for _, match := range malformedSimplifiedRegex.FindAllStringSubmatchIndex(content, -1) {
				lang := content[match[2]:match[3]]
				code := content[match[4]:match[5]]

				if lang == "js" {
					lang = "javascript"
				} else if lang == "py" {
					lang = "python"
				} else if lang == "sh" {
					lang = "bash"
				}

				blocksWithPos = append(blocksWithPos, blockWithPosition{
					block: CodeBlock{
						Language: lang,
						Code:     strings.TrimSpace(code),
					},
					pos: match[0],
				})
			}
		}

		// Incomplete markdown: ```python\n... (no closing backticks)
		if len(blocksWithPos) == 0 {
			incompleteMarkdownRegex := regexp.MustCompile("(?s)```(bash|python|javascript|js|sh|py|go)\\s*\\n(.+?)$")
			for _, match := range incompleteMarkdownRegex.FindAllStringSubmatchIndex(content, -1) {
				lang := content[match[2]:match[3]]
				code := content[match[4]:match[5]]

				if lang == "js" {
					lang = "javascript"
				} else if lang == "py" {
					lang = "python"
				} else if lang == "sh" {
					lang = "bash"
				}

				blocksWithPos = append(blocksWithPos, blockWithPosition{
					block: CodeBlock{
						Language: lang,
						Code:     strings.TrimSpace(code),
					},
					pos: match[0],
				})
			}
		}
	}

	// Sort by position and extract blocks (to maintain document order)
	// This isn't strictly necessary for most use cases, but maintains consistency
	var blocks []CodeBlock
	for _, bp := range blocksWithPos {
		blocks = append(blocks, bp.block)
	}

	return blocks
}

func StripCodeTags(content string) string {
	// Strip full XML format
	cleaned := regexp.MustCompile(`(?s)<code\s+language="[^"]+">.*?</code>`).ReplaceAllString(content, "")

	// Strip simplified XML format
	cleaned = regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)>.*?</code>`).ReplaceAllString(cleaned, "")

	// Strip markdown code blocks
	cleaned = regexp.MustCompile("(?s)```(?:bash|python|javascript|js|sh|py|go)\\s*\\n.*?```").ReplaceAllString(cleaned, "")

	// Strip incomplete full XML
	cleaned = regexp.MustCompile(`(?s)<code\s+language="[^"]+">.*`).ReplaceAllString(cleaned, "")

	// Strip incomplete simplified XML
	cleaned = regexp.MustCompile(`(?s)<code\s+(?:bash|python|javascript|js|sh|py|go)>.*`).ReplaceAllString(cleaned, "")

	// Strip incomplete markdown
	cleaned = regexp.MustCompile("(?s)```(?:bash|python|javascript|js|sh|py|go)\\s*\\n.*").ReplaceAllString(cleaned, "")

	return strings.TrimSpace(cleaned)
}

func StripTextBasedFunctionCalls(content string) string {
	functionCallRegex := regexp.MustCompile(`<function=[a-z_]+>?`)
	return functionCallRegex.ReplaceAllString(content, "")
}

func RenderWithSyntaxHighlighting(content string, maxWidth int, codeBlockBadge, codeBlockContainer lipgloss.Style) string {
	blocks := ParseCodeBlocks(content)

	codeContainerWidth := maxWidth - codeBlockContainer.GetHorizontalFrameSize()

	renderCodeBlock := func(language, code string) string {
		// Manually wrap code to fit within container width BEFORE passing to Glamour
		// This is necessary because Glamour doesn't wrap code blocks even with WithWordWrap
		maxLineLen := codeContainerWidth - 2 // Leave room for padding
		var wrappedLines []string
		for _, line := range strings.Split(code, "\n") {
			if len(line) <= maxLineLen {
				wrappedLines = append(wrappedLines, line)
			} else {
				// Hard wrap at maxLineLen
				remaining := line
				for len(remaining) > maxLineLen {
					wrappedLines = append(wrappedLines, remaining[:maxLineLen])
					remaining = remaining[maxLineLen:]
				}
				if len(remaining) > 0 {
					wrappedLines = append(wrappedLines, remaining)
				}
			}
		}
		// Reconstruct the markdown with wrapped code
		markdown := fmt.Sprintf("```%s\n%s\n```", language, strings.Join(wrappedLines, "\n"))

		// Render with glamour for syntax highlighting (wrapping already done)
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
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

		// Keep glamour's background colors - bubble containers apply their own backgrounds
		// which will work with the ANSI codes properly

		lines := strings.Split(rendered, "\n")
		var cleanLines []string
		for _, line := range lines {
			trimmed := strings.TrimRight(line, " ")
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
		// Replace full XML format
		fullXMLRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)</code>`)
		result = fullXMLRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := fullXMLRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			return renderCodeBlock(submatches[1], strings.TrimSpace(submatches[2]))
		})

		// Replace simplified XML format
		simplifiedXMLRegex := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)>(.+?)</code>`)
		result = simplifiedXMLRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := simplifiedXMLRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			lang := submatches[1]
			if lang == "js" {
				lang = "javascript"
			} else if lang == "py" {
				lang = "python"
			} else if lang == "sh" {
				lang = "bash"
			}
			return renderCodeBlock(lang, strings.TrimSpace(submatches[2]))
		})

		// Replace markdown code blocks
		markdownRegex := regexp.MustCompile("(?s)```(bash|python|javascript|js|sh|py|go)\\s*\\n(.+?)```")
		result = markdownRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := markdownRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			lang := submatches[1]
			if lang == "js" {
				lang = "javascript"
			} else if lang == "py" {
				lang = "python"
			} else if lang == "sh" {
				lang = "bash"
			}
			return renderCodeBlock(lang, strings.TrimSpace(submatches[2]))
		})

		// Replace incomplete full XML
		incompleteFullXMLRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)(?:</code|$)`)
		result = incompleteFullXMLRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := incompleteFullXMLRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			return renderCodeBlock(submatches[1], strings.TrimSpace(submatches[2]))
		})

		// Replace incomplete simplified XML
		incompleteSimplifiedRegex := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py|go)>(.+?)(?:</code|$)`)
		result = incompleteSimplifiedRegex.ReplaceAllStringFunc(result, func(match string) string {
			submatches := incompleteSimplifiedRegex.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}
			lang := submatches[1]
			if lang == "js" {
				lang = "javascript"
			} else if lang == "py" {
				lang = "python"
			} else if lang == "sh" {
				lang = "bash"
			}
			return renderCodeBlock(lang, strings.TrimSpace(submatches[2]))
		})
	}

	// Use "dark" style for markdown rendering with proper word wrap,
	// then strip background codes to avoid conflicts with message bubble backgrounds
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
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

	// Keep glamour's background colors - bubble containers apply their own backgrounds

	lines := strings.Split(rendered, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
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
