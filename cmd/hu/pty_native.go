//go:build !js

package main

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

// ptyOutput reads from a PTY master, forwards all output to os.Stdout, and
// tracks the last partial line (the prompt that the child process is
// displaying while waiting for input).
type ptyOutput struct {
	ptmx     *os.File
	lastLine string
	mu       sync.Mutex
	updated  chan struct{} // non-buffered signal: new data arrived
	done     chan struct{} // closed when read loop exits
}

func newCmd(args []string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command(args[0], args[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}
	return cmd, ptmx, nil
}

func newPtyOutput(ptmx *os.File) *ptyOutput {
	po := &ptyOutput{
		ptmx:    ptmx,
		updated: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	go po.readLoop()
	return po
}

func (po *ptyOutput) readLoop() {
	defer close(po.done)
	buf := make([]byte, 65536)
	var ll strings.Builder
	for {
		n, err := po.ptmx.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			po.mu.Lock()
			for _, b := range buf[:n] {
				if b == '\n' {
					ll.Reset()
				} else {
					ll.WriteByte(b)
				}
			}
			po.lastLine = ll.String()
			po.mu.Unlock()
			// Signal that new data arrived (non-blocking).
			select {
			case po.updated <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

func (po *ptyOutput) LastLine() string {
	po.mu.Lock()
	defer po.mu.Unlock()
	return po.lastLine
}

// WaitForPrompt blocks until the PTY output has been idle for idleTimeout
// and a partial line (the presumed prompt) has been captured. Returns ""
// if the child process exits or a global timeout (5 s) expires.
func (po *ptyOutput) WaitForPrompt(idleTimeout time.Duration) string {
	const maxWait = 5 * time.Second
	timer := time.NewTimer(maxWait)
	defer timer.Stop()

	for {
		ll := po.LastLine()
		if ll != "" {
			idle := time.NewTimer(idleTimeout)
			select {
			case <-po.updated:
				idle.Stop()
				continue
			case <-po.done:
				idle.Stop()
				return ll
			case <-idle.C:
				return ll
			}
		}
		// No data yet — wait for the first output.
		select {
		case <-po.updated:
			continue
		case <-po.done:
			return ""
		case <-timer.C:
			return ""
		}
	}
}
