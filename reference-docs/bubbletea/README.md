# Bubble Tea v1.3.6

The fun, functional and stateful way to build terminal apps. A Go framework based on [The Elm Architecture](https://guide.elm-lang.org/architecture/). Bubble Tea is well-suited for simple and complex terminal applications, either inline, full-window, or a mix of both.

![Bubble Tea Example](https://stuff.charm.sh/bubbletea/bubbletea-example.gif)

Bubble Tea is in use in production and includes a number of features and performance optimizations we’ve added along the way. Among those is a framerate-based renderer, mouse support, focus reporting and more.

To get started, see the tutorial below, the [examples](https://github.com/charmbracelet/bubbletea/tree/main/examples), the [docs](https://pkg.go.dev/github.com/charmbracelet/bubbletea?tab=doc), the [video tutorials](https://charm.sh/yt) and some common [resources](#libraries-we-use-with-bubble-tea).

---

## Tutorial

Bubble Tea is based on the functional design paradigms of [The Elm Architecture](https://guide.elm-lang.org/architecture/), which happens to work nicely with Go. It's a delightful way to build applications.

This tutorial assumes you have a working knowledge of Go.

The non-annotated source code for this program is available [on GitHub](https://github.com/charmbracelet/bubbletea/tree/main/tutorials/basics).

### Example: Shopping List

```go
package main

import (
    "fmt"
    "os"
    tea "github.com/charmbracelet/bubbletea"
)
```

Bubble Tea programs are comprised of a **model** that describes the application state and three simple methods on that model:

- **Init**: returns an initial command for the application to run.
- **Update**: handles incoming events and updates the model accordingly.
- **View**: renders the UI based on the data in the model.

#### The Model

```go
type model struct {
    choices  []string           // items on the to-do list
    cursor   int                // which to-do list item our cursor is pointing at
    selected map[int]struct{}   // which to-do items are selected
}
```

#### Initialization

```go
func initialModel() model {
    return model{
        choices:  []string{"Buy carrots", "Buy celery", "Buy kohlrabi"},
        selected: make(map[int]struct{}),
    }
}
```

#### The Init Method

```go
func (m model) Init() tea.Cmd {
    return nil
}
```

#### The Update Method

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.choices)-1 {
                m.cursor++
            }
        case "enter", " ":
            _, ok := m.selected[m.cursor]
            if ok {
                delete(m.selected, m.cursor)
            } else {
                m.selected[m.cursor] = struct{}{}
            }
        }
    }
    return m, nil
}
```

#### The View Method

```go
func (m model) View() string {
    s := "What should we buy at the market?\n\n"
    for i, choice := range m.choices {
        cursor := " "
        if m.cursor == i {
            cursor = ">"
        }
        checked := " "
        if _, ok := m.selected[i]; ok {
            checked = "x"
        }
        s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
    }
    s += "\nPress q to quit.\n"
    return s
}
```

#### All Together Now

```go
func main() {
    p := tea.NewProgram(initialModel())
    if _, err := p.Run(); err != nil {
        fmt.Printf("Alas, there's been an error: %v", err)
        os.Exit(1)
    }
}
```

---

## What’s Next?

- [Command Tutorial](https://github.com/charmbracelet/bubbletea/tree/main/tutorials/commands/)
- [Bubble Tea examples](https://github.com/charmbracelet/bubbletea/tree/main/examples)
- [Go Docs](https://pkg.go.dev/github.com/charmbracelet/bubbletea?tab=doc)

---

## Debugging

### Debugging with Delve

Since Bubble Tea apps assume control of stdin and stdout, you’ll need to run delve in headless mode and then connect to it:

```sh
# Start the debugger
$ dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .
API server listening at: 127.0.0.1:43000

# Connect to it from another terminal
$ dlv connect 127.0.0.1:43000
```

### Logging Stuff

You can’t really log to stdout with Bubble Tea because your TUI is busy occupying that! You can, however, log to a file:

```go
if len(os.Getenv("DEBUG")) > 0 {
    f, err := tea.LogToFile("debug.log", "debug")
    if err != nil {
        fmt.Println("fatal:", err)
        os.Exit(1)
    }
    defer f.Close()
}
```

To see what’s being logged in real time, run `tail -f debug.log` while you run your program in another window.

---

## Libraries we use with Bubble Tea

- [Bubbles](https://github.com/charmbracelet/bubbles): Common Bubble Tea components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss): Style, format and layout tools
- [Harmonica](https://github.com/charmbracelet/harmonica): Spring animation library
- [BubbleZone](https://github.com/lrstanley/bubblezone): Easy mouse event tracking
- [ntcharts](https://github.com/NimbleMarkets/ntcharts): Terminal charting library

---

## Bubble Tea in the Wild

There are over [10,000 applications](https://github.com/charmbracelet/bubbletea/network/dependents) built with Bubble Tea! Here are a handful:

### Staff favourites
- [chezmoi](https://github.com/twpayne/chezmoi)
- [circumflex](https://github.com/bensadeh/circumflex)
- [gh-dash](https://www.github.com/dlvhdr/gh-dash)
- [Tetrigo](https://github.com/Broderick-Westrope/tetrigo)
- [Signls](https://github.com/emprcl/signls)
- [Superfile](https://github.com/yorukot/superfile)

### In Industry
- Microsoft Azure – [Aztify](https://github.com/Azure/aztfy)
- Daytona – [Daytona](https://github.com/daytonaio/daytona)
- Cockroach Labs – [CockroachDB](https://github.com/cockroachdb/cockroach)
- Truffle Security Co. – [Trufflehog](https://github.com/trufflesecurity/trufflehog)
- NVIDIA – [container-canary](https://github.com/NVIDIA/container-canary)
- AWS – [eks-node-viewer](https://github.com/awslabs/eks-node-viewer)
- MinIO – [mc](https://github.com/minio/mc)
- Ubuntu – [Authd](https://github.com/ubuntu/authd)

### Charm stuff
- [Glow](https://github.com/charmbracelet/glow)
- [Huh?](https://github.com/charmbracelet/huh)
- [Mods](https://github.com/charmbracelet/mods)
- [Wishlist](https://github.com/charmbracelet/wishlist)

### There’s so much more where that came from
For more applications built with Bubble Tea see [Charm & Friends](https://github.com/charm-and-friends/charm-in-the-wild).

---

## Contributing
See [contributing](https://github.com/charmbracelet/bubbletea/contribute).

## Feedback
We’d love to hear your thoughts on this project. Feel free to drop us a note!
- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## Acknowledgments
Bubble Tea is based on the paradigms of [The Elm Architecture](https://guide.elm-lang.org/architecture/) and the excellent [go-tea](https://github.com/tj/go-tea) by TJ Holowaychuk.

## License
[MIT](https://github.com/charmbracelet/bubbletea/raw/main/LICENSE)

---

# Detailed Guide: Sizing and Layout of UI Elements in Bubble Tea

Bubble Tea uses [Lip Gloss](https://github.com/charmbracelet/lipgloss) for layout, sizing, and styling. For advanced UI elements (boxes, panels, tables, etc.), you may also use [Bubbles](https://github.com/charmbracelet/bubbles) components. This guide covers best practices, common pitfalls, and troubleshooting for sizing and layout.

## 1. Sizing Boxes and Panels

- **Set Width and Height Explicitly:**
  ```go
  style := lipgloss.NewStyle().Width(30).Height(10)
  fmt.Println(style.Render("My Box"))
  ```
  - `Width` and `Height` set the minimum size. Content will be padded if smaller.
  - If content is larger, it will overflow unless you use `MaxWidth`/`MaxHeight`.

- **Padding and Margin:**
  - Padding adds space inside the border; margin adds space outside.
  - Shorthand: `.Padding(2)` (all sides), `.Margin(1, 2)` (top/bottom, left/right), `.Padding(1, 2, 3, 4)` (top, right, bottom, left).

- **Borders:**
  - Add borders with `.BorderStyle(lipgloss.NormalBorder())` and set border color.
  - Borders take up space; account for border width when sizing.

## 2. Dynamic Sizing and Terminal Resize

- **Handle WindowSizeMsg:**
  - Bubble Tea sends `WindowSizeMsg` to your `Update` method on terminal resize.
  - Store the width/height in your model and use them to size your UI elements.
  ```go
  type model struct { width, height int }
  func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
      case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
    }
    return m, nil
  }
  ```
  - Use these values in your Lip Gloss styles: `.Width(m.width)`

## 3. Joining and Aligning Elements

- **JoinHorizontal/JoinVertical:**
  - Use `lipgloss.JoinHorizontal()` and `lipgloss.JoinVertical()` to combine boxes/panels.
  - Example:
    ```go
    left := lipgloss.NewStyle().Width(20).Render("Left")
    right := lipgloss.NewStyle().Width(20).Render("Right")
    row := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
    ```

- **Alignment:**
  - Use `.Align(lipgloss.Center)` to center content in a box.
  - For placing blocks in whitespace: `lipgloss.Place(width, height, hAlign, vAlign, content)`

## 4. Measuring and Debugging Size

- **Measure Rendered Size:**
  - Use `lipgloss.Width(str)` and `lipgloss.Height(str)` to get the actual size of a rendered block.
  - Use this to debug why boxes are not lining up as expected.

- **Unicode and Locale Issues:**
  - Misalignment often occurs due to Unicode width (CJK, emoji, etc.).
  - Set `RUNEWIDTH_EASTASIAN=0` in your environment to fix most issues.
  - See [Lip Gloss FAQ](https://github.com/charmbracelet/lipgloss/issues/40).

## 5. Common Pitfalls and Solutions

- **Borders not accounted for:** Always add border width to your box size.
- **Padding/margin confusion:** Padding is inside, margin is outside. Use both for spacing.
- **Terminal resizing ignored:** Always handle `WindowSizeMsg` for responsive layouts.
- **Content overflow:** Use `MaxWidth`/`MaxHeight` to clip content if needed.
- **Nested layouts:** When joining boxes, ensure each has explicit width/height for predictable results.

## 6. Advanced: Using Bubbles Components

- Bubbles provides ready-made components (table, list, viewport, etc.) that handle sizing internally, but you can style them with Lip Gloss.
- For custom boxes, always wrap content in a Lip Gloss style and set width/height.

## 7. Example: Responsive Two-Column Layout

```go
func (m model) View() string {
  left := lipgloss.NewStyle().Width(m.width/2-1).Render("Left Panel")
  right := lipgloss.NewStyle().Width(m.width/2-1).Render("Right Panel")
  return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}
```

## 8. Troubleshooting Checklist

- [ ] Are you setting width/height explicitly?
- [ ] Are you handling WindowSizeMsg?
- [ ] Are you using padding/margin correctly?
- [ ] Are you measuring rendered size with lipgloss.Width/Height?
- [ ] Is your locale/Unicode width correct?
- [ ] Are you joining blocks with JoinHorizontal/Vertical?
- [ ] Are you using borders and accounting for their width?

---

# Composable Views in Bubble Tea

Composable views allow you to build complex UIs by combining multiple independent Bubble Tea models (components) and switching focus or interaction between them. This pattern is useful for dashboards, multi-pane layouts, or any UI where you want to reuse and compose smaller, focused models.

## Key Concepts

- **Model Composition**: Your main model contains sub-models (e.g., timer, spinner) as fields.
- **Focus Management**: Track which sub-model is currently focused for input and updates.
- **Delegated Update**: Forward messages to the focused sub-model for handling.
- **View Composition**: Render each sub-model’s view and combine them using layout helpers (e.g., Lip Gloss).

## Step-by-Step Instructions

### 1. Define Sub-Models

Create fields in your main model for each sub-model you want to compose:

```go
type mainModel struct {
    state   sessionState // tracks which model is focused
    timer   timer.Model
    spinner spinner.Model
    index   int // for spinner selection
}
```

### 2. Track Focus

Use an enum or similar mechanism to track which sub-model is focused:

```go
type sessionState uint
const (
    timerView sessionState = iota
    spinnerView
)
```

### 3. Initialize Sub-Models

Initialize each sub-model in your main model’s constructor:

```go
func newModel(timeout time.Duration) mainModel {
    m := mainModel{state: timerView}
    m.timer = timer.New(timeout)
    m.spinner = spinner.New()
    return m
}
```

### 4. Delegate Updates

In your main model’s `Update` method, forward messages to the focused sub-model:

```go
func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    var cmds []tea.Cmd
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab":
            // Switch focus
            if m.state == timerView {
                m.state = spinnerView
            } else {
                m.state = timerView
            }
        }
        // Delegate update to focused model
        switch m.state {
        case spinnerView:
            m.spinner, cmd = m.spinner.Update(msg)
            cmds = append(cmds, cmd)
        default:
            m.timer, cmd = m.timer.Update(msg)
            cmds = append(cmds, cmd)
        }
    // ... handle other message types similarly
    }
    return m, tea.Batch(cmds...)
}
```

### 5. Compose Views

Render each sub-model’s view and combine them using layout helpers:

```go
func (m mainModel) View() string {
    var s string
    if m.state == timerView {
        s += lipgloss.JoinHorizontal(lipgloss.Top,
            focusedModelStyle.Render(m.timer.View()),
            modelStyle.Render(m.spinner.View()),
        )
    } else {
        s += lipgloss.JoinHorizontal(lipgloss.Top,
            modelStyle.Render(m.timer.View()),
            focusedModelStyle.Render(m.spinner.View()),
        )
    }
    s += helpStyle.Render("\ntab: focus next • n: new • q: exit\n")
    return s
}
```

### 6. Style and Layout

Use [Lip Gloss](https://github.com/charmbracelet/lipgloss) to style and arrange your views. You can highlight the focused model, set borders, and align content.

### 7. Extend with More Models

You can add more sub-models by following the same pattern: add a field, update focus logic, delegate updates, and compose views.

## Best Practices

- **Keep Sub-Models Independent**: Each sub-model should manage its own state and update logic.
- **Centralize Focus Logic**: The main model should control which sub-model receives input.
- **Batch Commands**: Use `tea.Batch` to run multiple commands from sub-models.
- **Use Layout Helpers**: Lip Gloss’s `JoinHorizontal` and `JoinVertical` are useful for arranging views.

## Example Use Cases

- Multi-pane dashboards
- Tabbed interfaces
- Wizards or step-by-step flows
- Split views (e.g., editor + preview)

---

**References:**
- [Bubble Tea composable-views example](https://github.com/charmbracelet/bubbletea/tree/main/examples/composable-views)
- [Lip Gloss documentation](https://github.com/charmbracelet/lipgloss)

If you follow these guidelines, you can build robust, composable UIs in Bubble Tea!

