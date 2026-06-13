package hush

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/fatih/color"
	"github.com/justwasm/bubbline"
)

const (
	promptTemplateStr = `{{.RCArrow}} {{.CurDirName}} $ `
)

var (
	promptTemplate = template.Must(template.New("").Parse(promptTemplateStr))
)

func promptErr(lastExitCode int) (string, error) {
	var buf bytes.Buffer
	data := newPromptData(lastExitCode)
	err := promptTemplate.Execute(&buf, data)
	return buf.String(), err
}

type promptData struct {
	RCArrow    string
	CurDirName string
}

func newPromptData(lastExitCode int) *promptData {
	const rcArrow = "➜"
	data := &promptData{
		RCArrow: color.GreenString(rcArrow),
	}

	wd, err := os.Getwd()
	if err != nil {
		return data
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return data
	}
	data.CurDirName = filepath.Base(wd)
	if wd == home {
		data.CurDirName = "~"
	}

	if lastExitCode != 0 {
		data.RCArrow = color.RedString(rcArrow)
	}

	return data
}

func updatePrompt(m *bubbline.Editor, lastExitCode int) {
	s, err := promptErr(lastExitCode)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to render prompt: ", err)
	}
	m.Prompt = s
}
