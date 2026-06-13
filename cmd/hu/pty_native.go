//go:build !js

package main

import (
	"io"
	"os/exec"

	"github.com/creack/pty"
)

func newCmd(args []string) (*exec.Cmd, io.WriteCloser, <-chan []byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, nil, err
	}
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				ch <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				break
			}
		}
	}()
	return cmd, ptmx, ch, nil
}
