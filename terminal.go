package hush

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/btwiuse/sh/v3/interp"
	"github.com/fatih/color"
)

const (
	escapeCSI      = '\x1B'
	escapeLBracket = '['
)

type terminal struct {
	// reader state
	line   []rune
	cursor int
	// command state
	lastExitCode int
	history      *history

	out, outErr io.Writer
	runner      *interp.Runner
}

func newTerminal(out, outErr io.Writer, runner *interp.Runner) *terminal {
	term := &terminal{
		out:    out,
		outErr: outErr,
		runner: runner,
	}
	history, err := newHistory()
	if err != nil {
		term.ErrPrint(color.RedString(err.Error()) + "\n")
	}
	term.history = history
	return term
}

func (t *terminal) Stdout() io.Writer {
	return t.out
}

func (t *terminal) Stderr() io.Writer {
	return t.outErr
}

func (t *terminal) Note() io.Writer {
	return io.Discard
}

func (t *terminal) Print(args ...interface{}) {
	fmt.Fprint(t.Stdout(), args...)
}

func (t *terminal) Printf(format string, args ...interface{}) {
	fmt.Fprintf(t.Stdout(), format, args...)
}

func (t *terminal) ErrPrint(args ...interface{}) {
	fmt.Fprint(t.Stderr(), args...)
}

func clearWriter(w io.Writer) {
	// TODO this wipes out some scrollback, need to figure out how to preserve it
	fmt.Fprint(w, string(escapeCSI)+"[H") // set cursor to top left
	fmt.Fprint(w, string(escapeCSI)+"[J") // clear viewport
}

// isKilledBySignal returns true if err is from a child process
// terminated by a signal (not a normal exit).
func isKilledBySignal(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	return ok && !exitErr.Exited()
}
