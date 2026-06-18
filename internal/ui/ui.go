// Package ui handles interactive prompting: confirmations, pick-lists, and
// free-text input with defaults. When stdin is not a terminal, prompts that
// require an answer fail loudly rather than guessing — matching the rule that
// scripted ambiguity is an error, never a guess.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// UI carries the streams and interactivity flag.
type UI struct {
	In          io.Reader
	Out         io.Writer
	Interactive bool
	reader      *bufio.Reader
}

// New wires a UI to the process streams, detecting whether stdin is a terminal.
func New() *UI {
	return &UI{In: os.Stdin, Out: os.Stdout, Interactive: isTTY(os.Stdin)}
}

// NewWith builds a UI over explicit streams (tests). Interactive is forced on so
// scripted input can drive prompts.
func NewWith(in io.Reader, out io.Writer) *UI {
	return &UI{In: in, Out: out, Interactive: true}
}

func (u *UI) buf() *bufio.Reader {
	if u.reader == nil {
		u.reader = bufio.NewReader(u.In)
	}
	return u.reader
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ErrNotInteractive is returned when a prompt is needed but stdin is not a tty.
var ErrNotInteractive = fmt.Errorf("input required but not running interactively")

// Confirm asks a yes/no question with a default.
func (u *UI) Confirm(prompt string, def bool) (bool, error) {
	if !u.Interactive {
		return def, nil
	}
	suffix := " [y/N] "
	if def {
		suffix = " [Y/n] "
	}
	fmt.Fprint(u.Out, prompt+suffix)
	line, err := u.buf().ReadString('\n')
	if err != nil && line == "" {
		return def, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

// Input asks for a line of text, returning def when the answer is empty.
func (u *UI) Input(prompt, def string) (string, error) {
	if !u.Interactive {
		if def != "" {
			return def, nil
		}
		return "", ErrNotInteractive
	}
	if def != "" {
		fmt.Fprintf(u.Out, "%s [%s]: ", prompt, def)
	} else {
		fmt.Fprintf(u.Out, "%s: ", prompt)
	}
	line, err := u.buf().ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

// Secret reads a line without echoing it (for tokens). On a real terminal it
// uses no-echo input; otherwise it falls back to a normal read so scripted tests
// still work.
func (u *UI) Secret(prompt string) (string, error) {
	if !u.Interactive {
		return "", ErrNotInteractive
	}
	fmt.Fprint(u.Out, prompt+": ")
	if f, ok := u.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(u.Out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := u.buf().ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Select shows a numbered pick-list and returns the chosen index. With a single
// option it returns 0 without prompting.
func (u *UI) Select(prompt string, labels []string) (int, error) {
	if len(labels) == 0 {
		return -1, fmt.Errorf("nothing to choose from")
	}
	if len(labels) == 1 {
		return 0, nil
	}
	if !u.Interactive {
		return -1, fmt.Errorf("%w: %s (candidates: %s)", ErrNotInteractive, prompt, strings.Join(labels, ", "))
	}
	fmt.Fprintln(u.Out, prompt)
	for i, l := range labels {
		fmt.Fprintf(u.Out, "  %d) %s\n", i+1, l)
	}
	for {
		fmt.Fprint(u.Out, "choose [1]: ")
		line, err := u.buf().ReadString('\n')
		if err != nil && line == "" {
			return -1, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(labels) {
			fmt.Fprintln(u.Out, "  invalid choice")
			continue
		}
		return n - 1, nil
	}
}
