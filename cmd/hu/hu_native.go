//go:build !js

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hu COMMAND [ARG]...")
		os.Exit(1)
	}

	args := os.Args[1:]

	cmd, ptmx, err := newCmd(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hu:", err)
		os.Exit(1)
	}
	defer ptmx.Close()

	exitCode := runEditor(ptmx)
	cmd.Wait()
	os.Exit(exitCode)
}

func runEditor(ptmx *os.File) int {
	po := newPtyOutput(ptmx)

	m := bubbline.New()
	m.ShowHelp = false
	m.CursorMode = cursor.CursorStatic

	home, err := os.UserHomeDir()
	if err == nil {
		histFile := filepath.Join(home, ".history")
		_ = m.LoadHistory(histFile)
		m.SetAutoSaveHistory(histFile, true)
	}

	m.AutoComplete = hushAutoComplete
	m.CheckInputComplete = checkInputComplete
	m.KeyMap.AlwaysNewline = key.NewBinding(
		key.WithKeys("ctrl+o", "ctrl+j"),
		key.WithHelp("C-o/C-j", "force newline"),
	)

	// Detect initial prompt from child process — this becomes the canonical
	// prompt string used for exact-match detection on ALL subsequent rounds.
	// Once set, it is never updated from detection results.
	canonicalPrompt := po.PassthroughUntilPrompt(ptmx, "")
	if canonicalPrompt == "" {
		canonicalPrompt = fallbackPrompt()
	}

	for {
		m.Prompt = canonicalPrompt
		val, err := m.GetLine(ProgramOptions...)
		if err != nil {
			if err == io.EOF {
				return 0
			}
			if errors.Is(err, bubbline.ErrInterrupted) {
				fmt.Println("^C")
				po.PassthroughUntilPrompt(ptmx, canonicalPrompt)
				// Ignore return — canonicalPrompt stays fixed.
				// If child exited, next write will fail.
				continue
			}
			if errors.Is(err, bubbline.ErrTerminated) {
				return 0
			}
			fmt.Fprintln(os.Stderr, color.RedString(err.Error()))
			continue
		}

		if val != "" {
			m.AddHistory(val)
		}

		// Send input to child process.
		_, err = io.WriteString(ptmx, val+"\n")
		if err != nil {
			return 0 // child exited
		}

		// Passthrough: forward user keystrokes to child (TUI programs like
		// less/htop work) and child output to stdout, until the child's
		// next prompt is detected via exact match against canonicalPrompt.
		next := po.PassthroughUntilPrompt(ptmx, canonicalPrompt)
		if next == "" {
			return 0
		}
	}
}
