//go:build js

package main

import (
	"io"
	"os/exec"
)

func newCmd(args []string) (*exec.Cmd, io.WriteCloser, <-chan []byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	ch := make(chan []byte)
	close(ch)
	return cmd, stdin, ch, nil
}
