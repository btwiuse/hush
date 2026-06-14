//go:build !js

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const (
	initialPromptTimeout = 5 * time.Second
	idleTimeout          = 100 * time.Millisecond
)

// ptyOutput reads from a PTY master, forwards all output to os.Stdout, and
// tracks the last partial line (used for prompt detection).
type ptyOutput struct {
	ptmx     *os.File
	lastLine string
	mu       sync.Mutex
	updated  chan struct{} // signal: new data arrived on ptmx
	done     chan struct{} // closed when read loop exits
}

func newCmd(args []string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command(args[0], args[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}
	watchWindowSize(ptmx)
	return cmd, ptmx, nil
}

// watchWindowSize listens for SIGWINCH and updates the PTY size to match
// the current terminal size, so child programs (less, vim, etc.) correctly
// handle terminal resizes.
func watchWindowSize(ptmx *os.File) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)

	// Apply initial size.
	syncWindowSize(ptmx)

	go func() {
		for range ch {
			syncWindowSize(ptmx)
		}
	}()
}

func syncWindowSize(ptmx *os.File) {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
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

// PassthroughUntilPrompt forwards I/O in both directions (stdin ↔ ptmx)
// until the child's prompt is detected.
//
// If canonical is empty, the first prompt is detected using an idle-timeout
// heuristic. Otherwise, output is matched exactly against the canonical
// prompt string, eliminating false triggers from TUI programs.
//
// Returns the detected prompt line, or "" if the child exits.
func (po *ptyOutput) PassthroughUntilPrompt(ptmx *os.File, canonical string) string {
	if canonical == "" {
		return po.detectInitialPrompt()
	}
	return po.matchPrompt(ptmx, canonical)
}

// detectInitialPrompt waits for output to go idle and returns the last
// partial line as the canonical prompt.
func (po *ptyOutput) detectInitialPrompt() string {
	const maxWait = initialPromptTimeout
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

// matchPrompt puts stdin in raw mode, forwards all user keystrokes to the
// child (so TUI programs work), and waits for ptmx output whose last
// partial line exactly equals the known canonical prompt.
func (po *ptyOutput) matchPrompt(ptmx *os.File, canonical string) string {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return po.detectInitialPrompt()
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Non-blocking stdin so we can check for user input without a
	// permanently-blocked goroutine (which would steal the first
	// keystroke from GetLine on the next cycle).
	fd := int(os.Stdin.Fd())
	syscall.SetNonblock(fd, true)
	defer syscall.SetNonblock(fd, false)

	poll := time.NewTimer(5 * time.Millisecond)
	defer poll.Stop()
	if !poll.Stop() {
		<-poll.C
	}

	for {
		// Non-blocking read from stdin — forward each byte to child.
		var b [1]byte
		n, err := syscall.Read(fd, b[:])
		if n > 0 {
			ptmx.Write(b[:n])
			continue
		}
		if err != syscall.EAGAIN && err != nil {
			return po.LastLine()
		}

		poll.Reset(5 * time.Millisecond)
		select {
		case <-po.updated:
			ll := po.LastLine()
			if ll == canonical {
				return ll
			}
		case <-po.done:
			return po.LastLine()
		case <-poll.C:
			// Resume loop and check stdin again.
		}
	}
}
