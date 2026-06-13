package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
	"github.com/justwasm/bubbline/editline"
)

var ProgramOptions = []tea.ProgramOption{
	tea.WithInput(os.Stdin),
	tea.WithOutput(os.Stdout),
}

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
	m.Prompt = prompt()
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

func hushAutoComplete(entireInput [][]rune, line, col int) (string, editline.Completions) {
	if line < 0 || line >= len(entireInput) {
		return "", nil
	}
	currentLine := entireInput[line]
	if col > len(currentLine) {
		col = len(currentLine)
	}

	start := col
	for start > 0 && currentLine[start-1] != ' ' && currentLine[start-1] != '\t' {
		start--
	}
	word := string(currentLine[start:col])

	var dir, filter string
	if idx := strings.LastIndex(word, "/"); idx >= 0 {
		dir = word[:idx+1]
		filter = word[idx+1:]
	} else {
		dir = "."
		filter = word
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filter) {
			continue
		}
		comp := fileJoin(dir, name)
		if e.IsDir() {
			comp += "/"
		}
		names = append(names, comp)
	}

	return "", editline.SimpleWordsCompletion(names, "files", col, start, col)
}

func fileJoin(dir, name string) string {
	if dir == "." {
		return name
	}
	if strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + name
	}
	return filepath.Join(dir, name)
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

func prompt() string {
	arrow := color.GreenString("➜")
	wd, err := os.Getwd()
	if err != nil {
		return arrow + " $ "
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return arrow + " " + filepath.Base(wd) + " $ "
	}
	dir := filepath.Base(wd)
	if wd == home {
		dir = "~"
	}
	return arrow + " " + dir + " $ "
}
