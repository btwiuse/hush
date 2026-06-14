package hush

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/btwiuse/sh/v3/interp"
)

const escapeCSI = '\x1B'

type repl struct {
	Console *Console
	runner  *interp.Runner
}

func newRepl(term *Console, runner *interp.Runner) *repl {
	return &repl{
		Console: term,
		runner:  runner,
	}
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
