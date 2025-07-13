# Gemini CLI UI Guide

This guide outlines how to build beautiful and effective terminal user interfaces (TUIs) for the Gemini CLI using the Bubble Tea ecosystem.

## Core Libraries

Our TUI development is based on the following libraries:

-   **[Bubble Tea](https://github.com/charmbracelet/bubbletea)**: The core framework for building terminal applications. It's based on the Elm architecture, which is a simple and elegant way to structure interactive programs.
-   **[Lip Gloss](https://github.com/charmbracelet/lipgloss)**: A library for styling terminal output with colors, borders, and layouts.
-   **[Bubbles](https://github.com/charmbracelet/bubbles)**: A collection of pre-built Bubble Tea components like spinners, text inputs, and paginators.

## The Bubble Tea Architecture

Bubble Tea applications are built around three main concepts:

-   **Model**: A struct that holds the state of your application.
-   **Init**: A function that returns the initial model and an initial command.
-   **Update**: A function that handles incoming messages (like key presses or timer ticks) and updates the model accordingly.
-   **View**: A function that renders the UI based on the current state of the model.

## Styling with Lip Gloss

Lip Gloss provides a simple and powerful way to style your UI. You can define styles with colors, padding, margins, and borders.

**Example:**

```go
import "github.com/charmbracelet/lipgloss"

var style = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#FAFAFA")).
    Background(lipgloss.Color("#7D56F4")).
    PaddingTop(2).
    PaddingLeft(4).
    Width(22)

fmt.Println(style.Render("Hello, kitty."))
```

## Layouts

For more complex layouts, you can use `lipgloss.JoinHorizontal` and `lipgloss.JoinVertical` to arrange styled text blocks.

**Example:**

```go
// Join two strings horizontally
lipgloss.JoinHorizontal(lipgloss.Top, "Hello", "World")

// Join two strings vertically
lipgloss.JoinVertical(lipgloss.Left, "Hello", "World")
```

## Essential Components from `bubbles`

The `bubbles` library provides a set of ready-to-use components that can be easily integrated into your application.

-   **`spinner`**: For showing loading or progress.
-   **`textinput`**: For capturing user input.
-   **`textarea`**: For multi-line text input.
-   **`list`**: For displaying and navigating lists of items.
-   **`table`**: For displaying tabular data.
-   **`progress`**: For displaying progress bars.
-   **`viewport`**: For creating scrollable areas.

## Creating a Debug View

To create a debug view, we can use a vertical split layout. The left pane will show the main application, and the right pane will show the logs.

We can use the `viewport` component from `bubbles` to create a scrollable log view. The `Update` function will be responsible for updating the log view with new log messages.

## Example: A Simple Debug View

Here's a conceptual example of how to create a debug view:

```go
package main

import (
    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type model struct {
    mainView  string
    logView   viewport.Model
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ... handle updates for mainView and logView
    return m, nil
}

func (m model) View() string {
    // Use lipgloss to join the mainView and logView horizontally
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        m.mainView,
        m.logView.View(),
    )
}
```

This guide provides a starting point for building TUIs with Bubble Tea. For more detailed information and examples, refer to the official documentation for each library.
