//go:build !js

package hush

import (
	"context"
	"fmt"
	"os"

	gotty "github.com/mattn/go-tty"
)

var globalTTY *gotty.TTY
var globalRawCancel func() error

func ttySetup() (context.CancelFunc, error) {
	tty, err := gotty.Open()
	if err != nil {
		return nil, err
	}
	rawCancel, err := tty.Raw()
	if err != nil {
		return nil, err
	}
	globalTTY = tty
	globalRawCancel = rawCancel
	os.Stdin = tty.Input()
	return func() {
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
	if globalTTY == nil || globalRawCancel == nil {
		return fn()
	}
	if err := globalRawCancel(); err != nil {
		return err
	}
	globalRawCancel = nil
	err := fn()
	rawCancel, rerr := globalTTY.Raw()
	if rerr != nil {
		fmt.Fprintln(os.Stderr, "Failed to re-enter raw mode:", rerr)
	} else {
		globalRawCancel = rawCancel
	}
	return err
}
