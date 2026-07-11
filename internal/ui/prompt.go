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
