//go:build js

package hush

import (
	"context"
	"os/exec"
)

func ttySetup() (context.CancelFunc, error) {
	return func() {}, nil
}

func ttyExitRawMode() {}

func ttyEnterRawMode() {}

func runForegroundExternal(cmd *exec.Cmd) error {
	return cmd.Run()
}
