package llm

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
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

func RenderWithSyntaxHighlighting(content string, maxWidth int) string {
	blocks := ParseCodeBlocks(content)

	hasCodeTag := strings.Contains(content, "<code")
	log.Printf("[SYNTAX-HIGHLIGHT] Found %d code blocks in content (maxWidth=%d, hasCodeTag=%v, contentLen=%d)", len(blocks), maxWidth, hasCodeTag, len(content))
	if hasCodeTag && len(blocks) == 0 {
		log.Printf("[SYNTAX-HIGHLIGHT] Content preview: %s", content[:min(200, len(content))])
	}

	if len(blocks) == 0 {
		return content
	}

	result := content

	// First, replace complete code blocks
	codeTagRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)</code>`)
	result = codeTagRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := codeTagRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		language := submatches[1]
		code := strings.TrimSpace(submatches[2])

		markdown := fmt.Sprintf("```%s\n%s\n```", language, code)
		log.Printf("[SYNTAX-HIGHLIGHT] Rendering %s code block with %d chars", language, len(code))

		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(maxWidth),
		)
		if err != nil {
			log.Printf("[SYNTAX-HIGHLIGHT] Failed to create glamour renderer: %v", err)
			return match
		}

		var buf bytes.Buffer
		rendered, err := renderer.Render(markdown)
		if err != nil {
			log.Printf("[SYNTAX-HIGHLIGHT] Failed to render markdown: %v", err)
			return match
		}

		log.Printf("[SYNTAX-HIGHLIGHT] Successfully rendered, output length: %d", len(rendered))
		buf.WriteString(rendered)
		return buf.String()
	})

	// Then, replace incomplete code blocks (missing closing tag or partial closing tag like "</code")
	incompleteRegex := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">(.+?)(?:</code|$)`)
	result = incompleteRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := incompleteRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		language := submatches[1]
		code := strings.TrimSpace(submatches[2])

		markdown := fmt.Sprintf("```%s\n%s\n```", language, code)
		log.Printf("[SYNTAX-HIGHLIGHT] Rendering incomplete %s code block with %d chars", language, len(code))

		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(maxWidth),
		)
		if err != nil {
			log.Printf("[SYNTAX-HIGHLIGHT] Failed to create glamour renderer: %v", err)
			return match
		}

		rendered, err := renderer.Render(markdown)
		if err != nil {
			log.Printf("[SYNTAX-HIGHLIGHT] Failed to render markdown: %v", err)
			return match
		}

		log.Printf("[SYNTAX-HIGHLIGHT] Successfully rendered incomplete block, output length: %d", len(rendered))
		return rendered
	})

	return result
}

