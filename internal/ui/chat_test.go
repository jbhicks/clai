package ui

import (
	"clai/internal/llm"
	uitesting "clai/internal/ui/testing"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func newTestChatModel() ChatModel {
	chatInput := textinput.New()
	chatInput.Prompt = "> "
	chatInput.Focus()

	spin := spinner.New()
	mockLLM := uitesting.NewMockLLM()
	theme := AvailableThemes[0] // Use Gruvbox theme

	return ChatModel{
		TextInput:    chatInput,
		Spinner:      spin,
		Theme:        theme,
		Viewport:     viewport.New(80, 20),
		Messages:     []llm.Message{},
		LlmClient:    mockLLM,
		Width:        80,
		Height:       20,
		ContentDirty: true,
	}
}

func TestChatModelViewEmpty(t *testing.T) {
	c := newTestChatModel()
	view := c.View()

	if view == "" {
		t.Error("expected non-empty view")
	}

	viewClean := uitesting.StripANSI(view)
	if !strings.Contains(viewClean, ">") {
		t.Error("expected prompt in view")
	}
}

func TestChatModelViewWithMessages(t *testing.T) {
	c := newTestChatModel()
	c.Messages = []llm.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	c.ContentDirty = true

	view := c.View()
	viewClean := uitesting.StripANSI(view)

	if !strings.Contains(viewClean, "Hello") {
		t.Error("expected user message in view")
	}

	if !strings.Contains(viewClean, "Hi there!") {
		t.Error("expected assistant message in view")
	}
}

func TestChatModelViewStreamingIndicator(t *testing.T) {
	c := newTestChatModel()
	c.Streaming = true

	view := c.View()
	viewClean := uitesting.StripANSI(view)

	if !strings.Contains(viewClean, "Generating") {
		t.Error("expected streaming indicator in view")
	}
}

func TestChatModelViewSelectingToolsIndicator(t *testing.T) {
	c := newTestChatModel()
	c.SelectingTools = true

	view := c.View()
	viewClean := uitesting.StripANSI(view)

	if !strings.Contains(viewClean, "Selecting tools") {
		t.Error("expected tool selection indicator in view")
	}
}

func TestChatModelViewToolBadge(t *testing.T) {
	c := newTestChatModel()
	c.Messages = []llm.Message{
		{Role: "user", Content: "Calculate 2+2"},
		{Role: "tool", Content: "calculator: 4"},
		{Role: "assistant", Content: "The result is 4", SelectedTools: []string{"calculator"}},
	}
	c.ContentDirty = true

	view := c.View()
	viewClean := uitesting.StripANSI(view)

	if !strings.Contains(viewClean, "calculator") {
		t.Error("expected tool badge in view")
	}
}

func TestChatModelViewSkipsToolMessages(t *testing.T) {
	c := newTestChatModel()
	c.Messages = []llm.Message{
		{Role: "user", Content: "test"},
		{Role: "tool", Content: "tool output"},
		{Role: "assistant", Content: "response"},
	}
	c.ContentDirty = true

	_ = c.View()

	if strings.Contains(c.CachedContent, "tool output") {
		t.Error("expected tool messages to be skipped in rendering")
	}
}

func TestChatModelViewCaching(t *testing.T) {
	c := newTestChatModel()
	c.Messages = []llm.Message{
		{Role: "user", Content: "Hello"},
	}
	c.ContentDirty = true

	view1 := c.View()
	cached1 := c.CachedContent

	if c.ContentDirty {
		t.Error("expected ContentDirty to be false after first render")
	}

	view2 := c.View()
	cached2 := c.CachedContent

	if cached1 != cached2 {
		t.Error("expected cached content to remain unchanged without dirty flag")
	}

	if view1 != view2 {
		t.Error("expected views to be identical when using cache")
	}
}

func TestChatModelViewDirtyFlagTriggersRebuild(t *testing.T) {
	c := newTestChatModel()
	c.Messages = []llm.Message{
		{Role: "user", Content: "Hello"},
	}
	c.ContentDirty = true

	_ = c.View()
	cached1 := c.CachedContent

	c.Messages = append(c.Messages, llm.Message{Role: "assistant", Content: "Hi"})
	c.ContentDirty = true

	_ = c.View()
	cached2 := c.CachedContent

	if cached1 == cached2 {
		t.Error("expected cached content to change when dirty flag set")
	}

	if !strings.Contains(cached2, "Hi") {
		t.Error("expected new message in cached content")
	}
}

func TestChatModelViewInputFocusedStyle(t *testing.T) {
	c := newTestChatModel()
	c.TextInput.Focus()

	view := c.View()

	if strings.Contains(view, "Ctrl+T") {
		t.Error("expected no tooltip when input focused")
	}
}

func TestChatModelViewInputUnfocusedTooltip(t *testing.T) {
	c := newTestChatModel()
	c.TextInput.Blur()

	view := c.View()
	viewClean := uitesting.StripANSI(view)

	if !strings.Contains(viewClean, "Ctrl") {
		t.Error("expected keyboard shortcuts tooltip when input unfocused")
	}
}

func TestChatModelViewDimensions(t *testing.T) {
	c := newTestChatModel()
	c.Width = 100
	c.Height = 30

	view := c.View()

	if view == "" {
		t.Error("expected non-empty view")
	}

	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Error("expected multiple lines in view")
	}
}
