package hush

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/btwiuse/sh/v3/interp"
)

const escapeCSI = '\x1B'

type terminal struct {
	out, outErr io.Writer
	runner      *interp.Runner
}

func newTerminal(out, outErr io.Writer, runner *interp.Runner) *terminal {
	term := &terminal{
		out:    out,
		outErr: outErr,
		runner: runner,
	}
	return term
}

func (t *terminal) Stdout() io.Writer {
	return t.out
}

func (t *terminal) Stderr() io.Writer {
	return t.outErr
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
