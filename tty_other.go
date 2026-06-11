//go:build !js

package hush

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

var (
	ttyFile  *os.File
	oldState *term.State
)

func ttySetup() (context.CancelFunc, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	old, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		f.Close()
		return nil, err
	}
	ttyFile = f
	oldState = old
	os.Stdin = f
	return func() {
		err := term.Restore(int(ttyFile.Fd()), oldState)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to restore tty:", err)
		}
		ttyFile.Close()
	}, nil
}

// ttyExitRawMode restores the terminal to cooked mode.
// Must be called before running an external command in the foreground.
func ttyExitRawMode() {
	if ttyFile != nil && oldState != nil {
		_ = term.Restore(int(ttyFile.Fd()), oldState)
	}
}

// ttyEnterRawMode re-enters raw mode after ttyExitRawMode.
func ttyEnterRawMode() {
	if ttyFile != nil {
		old, err := term.MakeRaw(int(ttyFile.Fd()))
		if err == nil {
			oldState = old
		}
	}
}

// runForegroundExternal runs an external command in the foreground.
//
// It temporarily restores the terminal to cooked mode so the child gets
// natural echo, line buffering, and Ctrl+C → SIGINT delivery (the child
// is in the same foreground process group).  SIGINT is intercepted so
// hush itself doesn't terminate.
//
// Before starting the child, stdout (and stderr) that were going through
// the carriageReturnWriter are replaced with the raw TTY fd.  This way
// the child sees a TTY on fd 1/2 and uses line buffering instead of the
// full buffering that Go exec pipes would trigger.
func runForegroundExternal(cmd *exec.Cmd) error {
	ttyExitRawMode()

	// Pass the TTY fd directly so the child sees a TTY (line buffered).
	// The terminal is now in cooked mode which handles CRLF translation.
	// If stdout/stderr were redirected (e.g. > file), the type assertion
	// fails and we leave them alone.
	if ttyFile != nil {
		if _, ok := cmd.Stdout.(*carriageReturnWriter); ok {
			cmd.Stdout = ttyFile
		}
		if _, ok := cmd.Stderr.(*carriageReturnWriter); ok {
			cmd.Stderr = ttyFile
		}
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	defer signal.Stop(sig)

	err := cmd.Run()

	ttyEnterRawMode()
	return err
}
