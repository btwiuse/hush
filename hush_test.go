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

func TestLn(t *testing.T) {
	t.Parallel()
	t.Run("ln -s creates symlink", func(t *testing.T) {
		t.Parallel()
		term := &redirectconsole{
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		dir := t.TempDir()
		target := dir + "/target"
		link := dir + "/link"

		if err := runLine(term, "touch "+target); err != nil {
			t.Fatalf("unexpected error creating target: %v", err)
		}
		if err := runLine(term, "ln -s "+target+" "+link); err != nil {
			t.Fatalf("unexpected error creating symlink: %v", err)
		}
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("failed to lstat symlink: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected symlink, got mode %v", info.Mode())
		}
		got, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if got != target {
			t.Errorf("expected symlink target %q, got %q", target, got)
		}
	})

	t.Run("ln without -s returns error", func(t *testing.T) {
		t.Parallel()
		term := &redirectconsole{
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		dir := t.TempDir()
		if err := runLine(term, "ln "+dir+" "+dir+"/link"); err == nil {
			t.Error("expected error for ln without -s")
		}
	})

	t.Run("ln -sf replaces existing file", func(t *testing.T) {
		t.Parallel()
		term := &redirectconsole{
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		dir := t.TempDir()
		target1 := dir + "/target1"
		target2 := dir + "/target2"
		link := dir + "/link"

		// Create two targets and an initial symlink
		if err := runLine(term, "touch "+target1); err != nil {
			t.Fatalf("unexpected error creating target1: %v", err)
		}
		if err := runLine(term, "touch "+target2); err != nil {
			t.Fatalf("unexpected error creating target2: %v", err)
		}
		if err := runLine(term, "ln -s "+target1+" "+link); err != nil {
			t.Fatalf("unexpected error creating first symlink: %v", err)
		}

		// Force override to point to target2
		if err := runLine(term, "ln -sf "+target2+" "+link); err != nil {
			t.Fatalf("unexpected error force-creating symlink: %v", err)
		}
		got, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if got != target2 {
			t.Errorf("expected symlink target %q after force, got %q", target2, got)
		}
	})

	t.Run("ln -s with 1 arg returns error", func(t *testing.T) {
		t.Parallel()
		term := &redirectconsole{
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		if err := runLine(term, "ln -s /tmp"); err == nil {
			t.Error("expected error for ln -s with 1 arg")
		}
	})
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
