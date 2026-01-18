package main

import (
	"fmt"
	"os"

	"github.com/jbhicks/clai/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
