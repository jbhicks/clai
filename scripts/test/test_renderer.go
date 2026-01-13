package main

import (
	"fmt"
	"github.com/charmbracelet/glamour"
)

func main() {
	// Test that glamour can render markdown
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	markdown := "**bold** and *italic* text"
	rendered, err := renderer.Render(markdown)

	if err != nil {
		fmt.Println("Error rendering markdown:", err)
		return
	}

	fmt.Println("Rendered output:")
	fmt.Println(rendered)
	fmt.Println("Raw markdown:")
	fmt.Println(markdown)
}
