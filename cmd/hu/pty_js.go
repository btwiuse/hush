//go:build js

package main

import (
	"io"
	"os/exec"
)

func newCmd(args []string) (*exec.Cmd, io.WriteCloser, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, stdin, nil
}
