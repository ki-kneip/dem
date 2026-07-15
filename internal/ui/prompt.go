package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// IsInteractive reports whether stdin and stdout are attached to a
// terminal. Commands should only prompt when this is true; otherwise
// they assume the default (scripts, CI, pipes).
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// Ask poses a question with a default value; an empty Enter returns
// the default.
func Ask(in io.Reader, out io.Writer, question, def string) (string, error) {
	fmt.Fprintf(out, "%s %s ", question, Dim("["+def+"]"))
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

// Confirm asks a yes/no question; only an explicit "y" or "yes"
// (case-insensitive) answers true, so a stray Enter on a destructive
// prompt defaults to declining rather than proceeding.
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	fmt.Fprintf(out, "%s %s ", question, Dim("[y/N]"))
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}
