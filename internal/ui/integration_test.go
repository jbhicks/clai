package ui

import (
	"bytes"
	"clai/internal/llm"
	uitesting "clai/internal/ui/testing"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestChatIntegrationSimpleConversation(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer

	mockLLM := uitesting.NewMockLLM()
	mockLLM.Response = "This is a test response"
	mockLLM.StreamChunks = []string{"This ", "is ", "a ", "test ", "response"}

	ti := textinput.New()
	ti.SetValue("test message")
	ti.Focus()

	m := &Model{
		Width:  80,
		Height: 24,
		Chat: ChatModel{
			Width:     80,
			Height:    20,
			LlmClient: mockLLM,
			Messages:  []llm.Message{},
			TextInput: ti,
			Viewport:  viewport.New(80, 15),
			Spinner:   spinner.New(),
			Theme:     AvailableThemes[0],
		},
		Theme: AvailableThemes[0],
		Agent: llm.NewAgent(mockLLM),
	}

	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(500 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		close(done)
	}()

	go func() {
		_, err := p.Run()
		if err != nil && ctx.Err() == nil {
			t.Errorf("program error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	if mockLLM.CallCount == 0 {
		t.Error("expected LLM to be called at least once")
	}
}

func TestChatIntegrationCodeExecution(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer

	baseMock := uitesting.NewMockLLM()
	callCount := 0

	mockLLM := &statefulMockLLM{
		base: baseMock,
		onStream: func(messages []llm.Message, streamChan chan<- string) (llm.Response, error) {
			callCount++
			if callCount == 1 {
				baseMock.Response = "```bash\necho 'test'\n```"
				baseMock.StreamChunks = []string{"```bash\necho 'test'\n```"}
			} else {
				baseMock.Response = "The result is test"
				baseMock.StreamChunks = []string{"The ", "result ", "is ", "test"}
			}
			return baseMock.SendMessageStreamNoTools(messages, streamChan, false)
		},
	}

	ti := textinput.New()
	ti.SetValue("run echo test")
	ti.Focus()

	m := &Model{
		Width:  80,
		Height: 24,
		Chat: ChatModel{
			Width:     80,
			Height:    20,
			LlmClient: mockLLM,
			Messages:  []llm.Message{},
			TextInput: ti,
			Viewport:  viewport.New(80, 15),
			Spinner:   spinner.New(),
			Theme:     AvailableThemes[0],
		},
		Theme: AvailableThemes[0],
		Agent: llm.NewAgent(mockLLM),
	}

	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(1 * time.Second)
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		close(done)
	}()

	go func() {
		_, err := p.Run()
		if err != nil && ctx.Err() == nil {
			t.Errorf("program error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	if baseMock.CallCount < 1 {
		t.Errorf("expected at least 1 LLM call, got %d", baseMock.CallCount)
	}
}

type statefulMockLLM struct {
	base     *uitesting.MockLLM
	onStream func([]llm.Message, chan<- string) (llm.Response, error)
}

func (s *statefulMockLLM) SendMessageStreamNoTools(messages []llm.Message, streamChan chan<- string, includeSystemPrompt bool) (llm.Response, error) {
	if s.onStream != nil {
		return s.onStream(messages, streamChan)
	}
	return s.base.SendMessageStreamNoTools(messages, streamChan, includeSystemPrompt)
}

func (s *statefulMockLLM) Model() string {
	return s.base.Model()
}

func (s *statefulMockLLM) Host() string {
	return s.base.Host()
}

func (s *statefulMockLLM) APIFormatString() string {
	return s.base.APIFormatString()
}

func TestChatIntegrationEmptyState(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer

	mockLLM := uitesting.NewMockLLM()

	m := &Model{
		Width:  80,
		Height: 24,
		Chat: ChatModel{
			Width:     80,
			Height:    20,
			LlmClient: mockLLM,
			Messages:  []llm.Message{},
			TextInput: textinput.New(),
			Viewport:  viewport.New(80, 15),
			Spinner:   spinner.New(),
			Theme:     AvailableThemes[0],
		},
		Theme: AvailableThemes[0],
		Agent: llm.NewAgent(mockLLM),
	}

	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		close(done)
	}()

	go func() {
		_, err := p.Run()
		if err != nil && ctx.Err() == nil {
			t.Errorf("program error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	if mockLLM.CallCount != 0 {
		t.Errorf("expected 0 LLM calls for empty state, got %d", mockLLM.CallCount)
	}
}

func TestChatIntegrationMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer

	mockLLM := uitesting.NewMockLLM()
	mockLLM.Response = "Response"
	mockLLM.StreamChunks = []string{"Resp", "onse"}

	ti := textinput.New()
	ti.SetValue("first message")
	ti.Focus()

	m := &Model{
		Width:  80,
		Height: 24,
		Chat: ChatModel{
			Width:     80,
			Height:    20,
			LlmClient: mockLLM,
			Messages:  []llm.Message{},
			TextInput: ti,
			Viewport:  viewport.New(80, 15),
			Spinner:   spinner.New(),
			Theme:     AvailableThemes[0],
		},
		Theme: AvailableThemes[0],
		Agent: llm.NewAgent(mockLLM),
	}

	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(300 * time.Millisecond)

		for _, r := range []rune("second message") {
			p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(300 * time.Millisecond)

		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		close(done)
	}()

	go func() {
		_, err := p.Run()
		if err != nil && ctx.Err() == nil {
			t.Errorf("program error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	if mockLLM.CallCount < 2 {
		t.Errorf("expected at least 2 LLM calls for 2 messages, got %d", mockLLM.CallCount)
	}
}

func TestChatIntegrationErrorHandling(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer

	mockLLM := uitesting.NewMockLLM()
	mockLLM.Error = errors.New("simulated LLM error")
	mockLLM.Response = ""

	ti := textinput.New()
	ti.SetValue("test message")
	ti.Focus()

	m := &Model{
		Width:  80,
		Height: 24,
		Chat: ChatModel{
			Width:     80,
			Height:    20,
			LlmClient: mockLLM,
			Messages:  []llm.Message{},
			TextInput: ti,
			Viewport:  viewport.New(80, 15),
			Spinner:   spinner.New(),
			Theme:     AvailableThemes[0],
		},
		Theme: AvailableThemes[0],
		Agent: llm.NewAgent(mockLLM),
	}

	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		time.Sleep(500 * time.Millisecond)
		p.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		close(done)
	}()

	go func() {
		_, err := p.Run()
		if err != nil && ctx.Err() == nil {
			t.Errorf("program error: %v", err)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	if mockLLM.CallCount == 0 {
		t.Error("expected LLM to be called at least once")
	}
}
