package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
	"github.com/justwasm/bubbline/editline"
)

var ProgramOptions = []tea.ProgramOption{}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hu COMMAND [ARG]...")
		os.Exit(1)
	}

	args := os.Args[1:]
	cmd := exec.Command(args[0], args[1:]...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hu:", err)
		os.Exit(1)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "hu:", err)
		os.Exit(1)
	}

	exitCode := runEditor(stdinPipe)
	stdinPipe.Close()
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
				return 0 // sh exited
			}
		}
	}
}

func hushAutoComplete(v [][]rune, line, col int) (string, editline.Completions) {
	return "", nil
}

func checkInputComplete(entireInput [][]rune, line, col int) bool {
	if line < 0 || line >= len(entireInput) {
		return true
	}
	currentLine := entireInput[line]
	if len(currentLine) == 0 {
		return true
	}
	return currentLine[len(currentLine)-1] != '\\'
}
