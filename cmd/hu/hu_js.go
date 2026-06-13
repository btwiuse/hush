//go:build js

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

	cmd, stdin, err := newCmd(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hu:", err)
		os.Exit(1)
	}

	exitCode := runEditor(stdin)
	stdin.Close()
	cmd.Wait()
	os.Exit(exitCode)
}

func runEditor(w io.Writer) int {
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
	m.Prompt = fallbackPrompt()
	m.KeyMap.AlwaysNewline = key.NewBinding(
		key.WithKeys("ctrl+o", "ctrl+j"),
		key.WithHelp("C-o/C-j", "force newline"),
	)

	for {
		val, err := m.GetLine(ProgramOptions...)
		if err != nil {
			if err == io.EOF {
				return 0
			}
			if errors.Is(err, bubbline.ErrInterrupted) {
				fmt.Println("^C")
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

		if val != "" {
			_, err := io.WriteString(w, val+"\n")
			if err != nil {
				return 0 // child exited
			}
		}
	}
}
