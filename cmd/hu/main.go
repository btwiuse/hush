package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/fatih/color"
	"github.com/justwasm/bubbline/editline"
)

var ProgramOptions = []tea.ProgramOption{
	tea.WithInput(os.Stdin),
	tea.WithOutput(os.Stdout),
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

func fallbackPrompt() string {
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
