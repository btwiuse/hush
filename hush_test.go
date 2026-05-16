package hush

import (
	"bytes"
	"os"
	"testing"

	"github.com/fatih/color"
)

func TestExport(t *testing.T) {
	t.Parallel()
	term := &redirectconsole{
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	if err := os.Unsetenv("HUSH_TEST_VAR"); err != nil {
		t.Fatalf("unexpected error unsetting env: %v", err)
	}
	if err := runLine(term, "export HUSH_TEST_VAR=hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("HUSH_TEST_VAR"); got != "hello" {
		t.Errorf("expected HUSH_TEST_VAR=hello, got %q", got)
	}

	// export VAR= sets to empty string
	if err := runLine(term, "export HUSH_TEST_VAR="); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("HUSH_TEST_VAR"); got != "" {
		t.Errorf("expected HUSH_TEST_VAR='', got %q", got)
	}

	// export VAR (naked) is a no-op for already-set variables
	if err := os.Setenv("HUSH_TEST_VAR", "world"); err != nil {
		t.Fatalf("unexpected error setting env: %v", err)
	}
	if err := runLine(term, "export HUSH_TEST_VAR"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := os.Getenv("HUSH_TEST_VAR"); got != "world" {
		t.Errorf("expected HUSH_TEST_VAR=world, got %q", got)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		input      string
		expectCode int
		expectOut  string
	}{
		{
			input:     "ls",
			expectOut: color.GreenString("➜") + " hush $ ls",
		},
	} {
		tc := tc // Enable parallel sub-tests
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			var in, out bytes.Buffer
			exitCode := run(&in, &out, &out, []string{"hush", "-c", tc.input})
			output := out.String()
			if tc.expectCode != exitCode {
				t.Errorf("Unexpected exit code.\nExpected: %d\nActual:  %d", tc.expectCode, exitCode)
			}
			if tc.expectOut != output {
				t.Errorf("Unexpected output.\nExpected: %s\nActual:   %s", tc.expectOut, output)
			}
		})
	}
}
