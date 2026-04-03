package term

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// EnsureTerminal checks that stdout is a terminal.
func EnsureTerminal() error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("stdout is not a terminal; termflix requires a TTY")
	}
	return nil
}

// EnableAltScreen switches to the alternate screen buffer.
func EnableAltScreen() error {
	_, err := os.Stdout.WriteString("\x1b[?1049h\x1b[H")
	return err
}

// Restore switches back to the normal screen buffer and resets attributes.
func Restore() error {
	_, err := os.Stdout.WriteString("\x1b[0m\x1b[?1049l")
	return err
}

