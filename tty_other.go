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
// When called between bubbline GetLine() calls, the terminal is already
// in cooked mode (bubbletea restored it), so ttyExitRawMode/Enter are
// effectively no-ops (ttyFile is nil when ttySetup was never called).
func runForegroundExternal(cmd *exec.Cmd) error {
	ttyExitRawMode()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	defer signal.Stop(sig)

	err := cmd.Run()

	ttyEnterRawMode()
	return err
}
