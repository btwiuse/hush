package hush

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/btwiuse/sh/v3/interp"
)

func testRunner(term console) *interp.Runner {
	var stdin io.Reader
	if stdiner, ok := term.(interface{ Stdin() io.Reader }); ok {
		stdin = stdiner.Stdin()
	}
	r, err := interp.New(
		interp.StdIO(stdin, term.Stdout(), term.Stderr()),
		interp.ExecHandlers(hushBuiltinMiddleware),
	)
	if err != nil {
		panic(err)
	}
	return r
}

func TestExport(t *testing.T) {
	t.Parallel()

	t.Run("export VAR=value", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &errOut,
		}
		runner := testRunner(term)

		if err := runLine(runner, "export HUSH_TEST_VAR=hello"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify via echo that the var is visible in the interp environment
		if err := runLine(runner, "echo $HUSH_TEST_VAR"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "hello") {
			t.Errorf("expected HUSH_TEST_VAR=hello in output, got %q", out.String())
		}
	})

	t.Run("export VAR= sets empty", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &errOut,
		}
		runner := testRunner(term)

		if err := runLine(runner, "export HUSH_TEST_VAR=hello"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := runLine(runner, "export HUSH_TEST_VAR="); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out.Reset()
		if err := runLine(runner, "echo $HUSH_TEST_VAR"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := strings.TrimSpace(out.String()); got != "" {
			t.Errorf("expected HUSH_TEST_VAR='', got %q", got)
		}
	})
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
		runner := testRunner(term)

		dir := t.TempDir()
		target := dir + "/target"
		link := dir + "/link"

		if err := runLine(runner, "touch "+target); err != nil {
			t.Fatalf("unexpected error creating target: %v", err)
		}
		if err := runLine(runner, "ln -s "+target+" "+link); err != nil {
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
		runner := testRunner(term)

		dir := t.TempDir()
		if err := runLine(runner, "ln "+dir+" "+dir+"/link"); err == nil {
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
		runner := testRunner(term)

		dir := t.TempDir()
		target1 := dir + "/target1"
		target2 := dir + "/target2"
		link := dir + "/link"

		// Create two targets and an initial symlink
		if err := runLine(runner, "touch "+target1); err != nil {
			t.Fatalf("unexpected error creating target1: %v", err)
		}
		if err := runLine(runner, "touch "+target2); err != nil {
			t.Fatalf("unexpected error creating target2: %v", err)
		}
		if err := runLine(runner, "ln -s "+target1+" "+link); err != nil {
			t.Fatalf("unexpected error creating first symlink: %v", err)
		}

		// Force override to point to target2
		if err := runLine(runner, "ln -sf "+target2+" "+link); err != nil {
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
		runner := testRunner(term)

		if err := runLine(runner, "ln -s /tmp"); err == nil {
			t.Error("expected error for ln -s with 1 arg")
		}
	})
}

func TestCurl(t *testing.T) {
	t.Parallel()

	t.Run("curl GET body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello world"))
		}))
		defer ts.Close()

		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term, ts.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "hello world") {
			t.Errorf("expected body in output, got: %s", out.String())
		}
	})

	t.Run("curl -I headers only", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello world"))
		}))
		defer ts.Close()

		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term, "-I", ts.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "200 OK") {
			t.Errorf("expected 200 OK in output, got: %s", out.String())
		}
	})

	t.Run("curl with no URL", func(t *testing.T) {
		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term); err == nil {
			t.Error("expected error for no URL")
		}
	})

	t.Run("curl -O saves to file", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello world"))
		}))
		defer ts.Close()

		dir := t.TempDir()
		oldWd, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(oldWd)

		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term, "-O", ts.URL+"/testfile.txt"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(dir + "/testfile.txt")
		if err != nil {
			t.Fatalf("failed to read saved file: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("curl -L follows redirects", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello world"))
		}))
		defer target.Close()

		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		defer redirectServer.Close()

		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term, "-L", redirectServer.URL); err != nil {
			t.Fatalf("unexpected error following redirect: %v", err)
		}
		if !strings.Contains(out.String(), "hello world") {
			t.Errorf("expected body after redirect, got: %s", out.String())
		}
	})

	t.Run("curl without -L does not follow redirect", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("should not reach"))
		}))
		defer target.Close()

		redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}))
		defer redirectServer.Close()

		var out bytes.Buffer
		term := &redirectconsole{
			stdout: &out,
			stderr: &bytes.Buffer{},
		}
		if err := curl(term, redirectServer.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.String(), "should not reach") {
			t.Error("expected NOT to follow redirect without -L")
		}
	})
}

func TestRun(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		input      string
		expectCode int
		// expectOut is not checked for -c mode in the interp-based shell,
		// since -c no longer goes through the REPL prompt.
	}{
		{
			input:      "ls",
			expectCode: 0,
		},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			var in, out bytes.Buffer
			exitCode := run(&in, &out, &out, []string{"hush", "-c", tc.input})
			_ = out.String()
			if tc.expectCode != exitCode {
				t.Errorf("Unexpected exit code.\nExpected: %d\nActual:  %d", tc.expectCode, exitCode)
			}
		})
	}
}
