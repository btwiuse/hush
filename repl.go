package hush

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/btwiuse/sh/v3/interp"
	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
	"github.com/justwasm/bubbline/editline"
)

var ProgramOptions = []tea.ProgramOption{}

// exitCodeFromError extracts the exit code from an error returned by
// runner.Run/runLine. Returns 0 on nil.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var es interp.ExitStatus
	if errors.As(err, &es) {
		return int(es)
	}
	if isKilledBySignal(err) {
		return 130
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func (t *terminal) bubblineReadEvalPrintLoop() int {
	m := bubbline.New()
	m.ShowHelp = false

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

	var lastExitCode int
	for {
		updatePrompt(m, lastExitCode)

		val, err := m.GetLine(ProgramOptions...)

		if err != nil {
			if err == io.EOF {
				return 0
			}
			if errors.Is(err, bubbline.ErrInterrupted) {
				fmt.Println("^C")
				lastExitCode = 1
				continue
			}
			if errors.Is(err, bubbline.ErrTerminated) {
				return 0
			}
			t.ErrPrint(color.RedString(err.Error()) + "\n")
			lastExitCode = 1
			continue
		}

		if val != "" {
			m.AddHistory(val)
		}

		if val != "" {
			err = runLine(t.runner, t, val)
			lastExitCode = exitCodeFromError(err)
			if err != nil {
				var es interp.ExitStatus
				if errors.As(err, &es) {
					// ExitStatus is the normal non-zero exit path; already captured.
				} else if isKilledBySignal(err) {
					fmt.Println("^C")
				} else {
					t.ErrPrint(color.RedString(err.Error()) + "\n")
				}
			}
			if t.runner.Exited() {
				return lastExitCode
			}
		}
	}
}

func hushAutoComplete(v [][]rune, line, col int) (string, editline.Completions) {
	var sb strings.Builder
	for _, l := range v {
		for _, r := range l {
			sb.WriteRune(r)
		}
	}
	input := sb.String()

	completions := getCompletions(input, col)
	if len(completions) == 0 {
		return "", nil
	}

	words := make([]string, len(completions))
	for i, c := range completions {
		words[i] = c.Completion
	}

	start := completions[0].Start
	end := completions[0].End

	return "", editline.SimpleWordsCompletion(words, "completions", col, start, end)
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
