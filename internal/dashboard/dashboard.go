package dashboard

import (
	"errors"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Run opens the dashboard and returns when it is closed.
func Run() error {
	// A full-screen interface needs a terminal on both ends: one to draw on,
	// and one to read keys from. Saying so is better than drawing escape
	// codes into a pipe.
	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("the dashboard needs a terminal; use 'solitary ls' from a script")
	}

	_, err := tea.NewProgram(newModel(), tea.WithAltScreen()).Run()

	return err
}
