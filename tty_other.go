//go:build !js

package hush

import (
	"context"
	"fmt"
	"os"
	"sync"

	gotty "github.com/mattn/go-tty"
)

var (
	rawMu           sync.Mutex
	globalTTY       *gotty.TTY
	globalRawCancel func() error
)

// suspendRawMode restores the terminal to cooked mode so that foreground
// subprocesses receive a normal terminal (echo, signal generation, etc.).
func suspendRawMode() {
	rawMu.Lock()
	defer rawMu.Unlock()
	if globalRawCancel != nil {
		globalRawCancel() //nolint:errcheck
		globalRawCancel = nil
	}
}

// resumeRawMode puts the terminal back into raw mode after a foreground
// subprocess has exited.
func resumeRawMode() {
	rawMu.Lock()
	defer rawMu.Unlock()
	if globalTTY != nil && globalRawCancel == nil {
		cancel, err := globalTTY.Raw()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to restore raw mode:", err)
			return
		}
		globalRawCancel = cancel
	}
}

func ttySetup() (context.CancelFunc, error) {
	tty, err := gotty.Open()
	if err != nil {
		return nil, err
	}
	cancel, err := tty.Raw()
	if err != nil {
		return nil, err
	}
	rawMu.Lock()
	globalTTY = tty
	globalRawCancel = cancel
	rawMu.Unlock()
	os.Stdin = tty.Input()
	return func() {
		suspendRawMode()
		tty.Close()
	}, nil
}
