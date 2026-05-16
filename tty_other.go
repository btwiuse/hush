//go:build !js

package hush

import (
	"context"
	"fmt"
	"os"

	gotty "github.com/mattn/go-tty"
)

func ttySetup() (context.CancelFunc, error) {
	tty, err := gotty.Open()
	if err != nil {
		return nil, err
	}
	cancel, err := tty.Raw()
	if err != nil {
		return nil, err
	}
	// go-tty v0.0.8+ calls syscall.SetNonblock on the tty fd after file.Fd() detaches
	// it from Go's I/O poller. This causes file.Read() to return EAGAIN immediately
	// instead of blocking for input. Re-open /dev/tty to get a fresh fd that Go's
	// runtime properly manages via its poller. Terminal settings (raw mode) are
	// per-terminal-device and apply to all fds pointing to the same terminal.
	ttyFile, err := os.Open("/dev/tty")
	if err != nil {
		cancel()
		tty.Close()
		return nil, err
	}
	os.Stdin = ttyFile
	return func() {
		ttyFile.Close()
		err := cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to restore tty:", err)
		}
		tty.Close()
	}, nil
}
