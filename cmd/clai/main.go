package main

import (
	"os"
	"runtime/debug"
	"strings"
)

// Build information - set at build time with -ldflags
var (
	buildTime  string
	gitCommit  string
	buildCount string
	buildRand  string
)

func getStackTrace() string {
	return string(debug.Stack())
}

func getBuildIdentifier() string {
	// Build identifier for restart verification
	parts := []string{}

	if buildTime != "" {
		parts = append(parts, buildTime)
	}
	if gitCommit != "" {
		// Truncate commit hash to first 7 characters for display
		if len(gitCommit) > 7 {
			parts = append(parts, gitCommit[:7])
		} else {
			parts = append(parts, gitCommit)
		}
	}
	if buildCount != "" {
		parts = append(parts, "b"+buildCount)
	}
	if buildRand != "" {
		parts = append(parts, buildRand)
	}

	if len(parts) == 0 {
		return "dev"
	}

	return strings.Join(parts, "-")
}

func main() {
	// Check for subcommands first - these should run without TUI setup
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "debug":
			runDebugCommand(os.Args[2:])
			return
		case "models":
			runModelsCommand(os.Args[2:])
			return
		case "ask":
			runAskCommand(os.Args[2:])
			return
		case "benchmark":
			runBenchmarkCommand(os.Args[2:])
			return
		case "service":
			runServiceCommand(os.Args[2:])
			return
		case "stop":
			runStopCommand(os.Args[2:])
			return
		case "tui":
			runTUICommand(os.Args[2:])
			return
		}
	}

	// Default to TUI mode
	runTUICommand(os.Args[1:])
}
