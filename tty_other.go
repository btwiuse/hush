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
	globalTTY        *gotty.TTY
	globalRawCancel  func() error
	globalTerminalMu sync.Mutex
)

func ttySetup() (context.CancelFunc, error) {
	tty, err := gotty.Open()
	if err != nil {
		return nil, err
	}
	rawCancel, err := tty.Raw()
	if err != nil {
		return nil, err
	}
	globalTerminalMu.Lock()
	globalTTY = tty
	globalRawCancel = rawCancel
	globalTerminalMu.Unlock()
	os.Stdin = tty.Input()
	return func() {
		globalTerminalMu.Lock()
		defer globalTerminalMu.Unlock()
		if globalRawCancel != nil {
			if err := globalRawCancel(); err != nil {
				fmt.Fprintln(os.Stderr, "Failed to restore tty:", err)
			}
			globalRawCancel = nil
		}
		globalTTY = nil
		tty.Close()
	}, nil
}

// withCookedMode temporarily restores the terminal to cooked mode while fn runs,
// then re-enters raw mode. This allows external commands to receive signals
// (e.g. Ctrl+C → SIGINT) and proper line-editing from the terminal driver.
func withCookedMode(fn func() error) error {
	globalTerminalMu.Lock()
	tty := globalTTY
	rawCancel := globalRawCancel
	globalTerminalMu.Unlock()

	if tty == nil || rawCancel == nil {
		return fn()
	}
	if err := rawCancel(); err != nil {
		return fmt.Errorf("restoring cooked terminal mode: %w", err)
	}
	globalTerminalMu.Lock()
	globalRawCancel = nil
	globalTerminalMu.Unlock()

	err := fn()

	newCancel, rerr := tty.Raw()
	if rerr != nil {
		fmt.Fprintln(os.Stderr, "Failed to re-enter raw mode:", rerr)
	} else {
		globalTerminalMu.Lock()
		globalRawCancel = newCancel
		globalTerminalMu.Unlock()
	}
	return err
}
