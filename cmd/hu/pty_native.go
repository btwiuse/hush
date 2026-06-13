//go:build !js

package main

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func newCmd(args []string) (*exec.Cmd, io.WriteCloser, error) {
	cmd := exec.Command(args[0], args[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}
	go io.Copy(os.Stdout, ptmx)
	return cmd, ptmx, nil
}
