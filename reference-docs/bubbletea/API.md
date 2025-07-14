# Bubble Tea v1.3.6 API Reference

## Overview

Package `tea` provides a framework for building rich terminal user interfaces based on the paradigms of The Elm Architecture. It's well-suited for simple and complex terminal applications, either inline, full-window, or a mix of both. It's been battle-tested in several large projects and is production-ready.

- Tutorial: [https://github.com/charmbracelet/bubbletea/tree/master/tutorials](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- Examples: [https://github.com/charmbracelet/bubbletea/tree/master/examples](https://github.com/charmbracelet/bubbletea/tree/master/examples)

---

## Variables

```go
var ErrInterrupted = errors.New("program was interrupted")
var ErrProgramKilled = errors.New("program was killed")
var ErrProgramPanic = errors.New("program experienced a panic")
```

---

## Functions

### Logging
```go
func LogToFile(path string, prefix string) (*os.File, error)
func LogToFileWith(path string, prefix string, log LogOptionsSetter) (*os.File, error)
```

### Commands
```go
func Batch(cmds ...Cmd) Cmd
func Every(duration time.Duration, fn func(time.Time) Msg) Cmd
func Exec(c ExecCommand, fn ExecCallback) Cmd
func ExecProcess(c *exec.Cmd, fn ExecCallback) Cmd
func Printf(template string, args ...interface{}) Cmd
func Println(args ...interface{}) Cmd
func ScrollDown(newLines []string, topBoundary, bottomBoundary int) Cmd // deprecated
func ScrollUp(newLines []string, topBoundary, bottomBoundary int) Cmd // deprecated
func Sequence(cmds ...Cmd) Cmd
func Sequentially(cmds ...Cmd) Cmd // deprecated
func SetWindowTitle(title string) Cmd
func SyncScrollArea(lines []string, topBoundary int, bottomBoundary int) Cmd // deprecated
func Tick(d time.Duration, fn func(time.Time) Msg) Cmd
func WindowSize() Cmd
```

### Msg Functions
```go
func ClearScreen() Msg
func ClearScrollArea() Msg // deprecated
func DisableBracketedPaste() Msg
func DisableMouse() Msg
func DisableReportFocus() Msg
func EnableBracketedPaste() Msg
func EnableMouseAllMotion() Msg
func EnableMouseCellMotion() Msg
func EnableReportFocus() Msg
func EnterAltScreen() Msg
func ExitAltScreen() Msg
func HideCursor() Msg
func Interrupt() Msg
func Quit() Msg
func ShowCursor() Msg
func Suspend() Msg
```

---

## Types

### Core Types
```go
type Cmd func() Msg

type Model interface {
    Init() Cmd
    Update(Msg) (Model, Cmd)
    View() string
}

type Msg interface{}
```

### Messages
```go
type BatchMsg []Cmd
type BlurMsg struct{}
type FocusMsg struct{}
type InterruptMsg struct{}
type QuitMsg struct{}
type ResumeMsg struct{}
type SuspendMsg struct{}
type WindowSizeMsg struct {
    Width  int
    Height int
}
```

### Key Handling
```go
type Key struct {
    Type  KeyType
    Runes []rune
    Alt   bool
    Paste bool
}
func (k Key) String() string

type KeyMsg Key
func (k KeyMsg) String() string

type KeyType int
func (k KeyType) String() string
```

#### KeyType Constants
```go
const (
    KeyNull      KeyType = ...
    KeyBreak     KeyType = ...
    KeyEnter     KeyType = ...
    KeyBackspace KeyType = ...
    KeyTab       KeyType = ...
    KeyEsc       KeyType = ...
    KeyEscape    KeyType = ...
    KeyCtrlAt           KeyType = ... // ctrl+@
    KeyCtrlA            KeyType = ...
    // ... (many more)
    KeyRunes KeyType = -(iota + 1)
    KeyUp
    KeyDown
    KeyRight
    KeyLeft
    // ... (many more)
)
```

### Mouse Handling
```go
type MouseAction int
const (
    MouseActionPress MouseAction = iota
    MouseActionRelease
    MouseActionMotion
)

type MouseButton int
const (
    MouseButtonNone MouseButton = iota
    MouseButtonLeft
    MouseButtonMiddle
    MouseButtonRight
    MouseButtonWheelUp
    MouseButtonWheelDown
    MouseButtonWheelLeft
    MouseButtonWheelRight
    MouseButtonBackward
    MouseButtonForward
    MouseButton10
    MouseButton11
)

type MouseEvent struct {
    X      int
    Y      int
    Shift  bool
    Alt    bool
    Ctrl   bool
    Action MouseAction
    Button MouseButton
    Type   MouseEventType // deprecated
}
func (m MouseEvent) IsWheel() bool
func (m MouseEvent) String() string

type MouseMsg MouseEvent
func (m MouseMsg) String() string

type MouseEventType int // deprecated
```

### Program
```go
type Program struct { /* ... */ }
func NewProgram(model Model, opts ...ProgramOption) *Program
func (p *Program) Kill()
func (p *Program) Printf(template string, args ...interface{})
func (p *Program) Println(args ...interface{})
func (p *Program) Quit()
func (p *Program) ReleaseTerminal() error
func (p *Program) RestoreTerminal() error
func (p *Program) Run() (returnModel Model, returnErr error)
func (p *Program) Send(msg Msg)
func (p *Program) Wait()
```

### Program Options
```go
type ProgramOption func(*Program)
func WithAltScreen() ProgramOption
func WithContext(ctx context.Context) ProgramOption
func WithEnvironment(env []string) ProgramOption
func WithFPS(fps int) ProgramOption
func WithFilter(filter func(Model, Msg) Msg) ProgramOption
func WithInput(input io.Reader) ProgramOption
func WithInputTTY() ProgramOption
func WithMouseAllMotion() ProgramOption
func WithMouseCellMotion() ProgramOption
func WithOutput(output io.Writer) ProgramOption
func WithReportFocus() ProgramOption
func WithoutBracketedPaste() ProgramOption
func WithoutCatchPanics() ProgramOption
func WithoutRenderer() ProgramOption
func WithoutSignalHandler() ProgramOption
func WithoutSignals() ProgramOption
```

### Logging
```go
type LogOptionsSetter interface {
    SetOutput(io.Writer)
    SetPrefix(string)
}
```

---

## Source Files

- commands.go
- exec.go
- focus.go
- key.go
- logging.go
- mouse.go
- options.go
- renderer.go
- screen.go
- standard_renderer.go
- tea.go
- tea_init.go
- tty.go

For full source, see: [https://github.com/charmbracelet/bubbletea/tree/v1.3.6](https://github.com/charmbracelet/bubbletea/tree/v1.3.6)
