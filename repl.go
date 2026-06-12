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
	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
	"github.com/justwasm/bubbline/editline"
	pkgerrors "github.com/pkg/errors"
)

func (t *terminal) bubblineReadEvalPrintLoop() int {
	m := bubbline.New()

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
		updatePrompt(m, t.lastExitCode)

		val, err := m.GetLine()

		if err != nil {
			if err == io.EOF {
				return 0
			}
			if errors.Is(err, bubbline.ErrInterrupted) {
				fmt.Println("^C")
				t.lastExitCode = 1
				continue
			}
			if errors.Is(err, bubbline.ErrTerminated) {
				return 0
			}
			t.ErrPrint(color.RedString(err.Error()) + "\n")
			t.lastExitCode = 1
			continue
		}

		if val != "" {
			m.AddHistory(val)
		}

		if val != "" {
			err = runLine(t, val)
			t.lastExitCode = 0
			if err != nil {
				if exitErr, ok := pkgerrors.Cause(err).(*exitErr); ok {
					return exitErr.Code
				}
				if isKilledBySignal(err) {
					fmt.Println("^C")
					t.lastExitCode = 130
				} else {
					t.ErrPrint(color.RedString(err.Error()) + "\n")
					t.lastExitCode = 1
					if exitErr, ok := err.(*exec.ExitError); ok {
						t.lastExitCode = exitErr.ExitCode()
					}
				}
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
