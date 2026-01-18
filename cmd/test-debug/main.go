package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jbhicks/clai/internal/debug"
)

type simpleModel struct{}

func (m simpleModel) Init() tea.Cmd {
	return nil
}

func (m simpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m simpleModel) View() string {
	return "CLAI Debug Server Test - Press Ctrl+C to stop\n"
}

func main() {
	model := simpleModel{}
	p := tea.NewProgram(model, tea.WithAltScreen())

	debugServer := debug.NewServer(p)
	if err := debugServer.Start(); err != nil {
		log.Fatalf("Failed to start debug server: %v", err)
	}

	log.Println("Debug server started, press Ctrl+C to stop")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down debug server...")
		debugServer.Stop()
		p.Quit()
	}()

	select {}
}
