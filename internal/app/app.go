package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kyaoi/mdview/internal/ui"
)

// Run executes the Bubble Tea program for the markdown viewer.
func Run(target string) error {
	state, err := LoadInitialState(target)
	if err != nil {
		return err
	}
	return runProgram(state)
}

func runProgram(state ui.State) error {
	program := tea.NewProgram(ui.NewModel(state), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
