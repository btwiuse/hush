package hush

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/btwiuse/sh/v3/expand"
	"github.com/btwiuse/sh/v3/interp"
	"github.com/btwiuse/sh/v3/syntax"
	"github.com/pkg/errors"
)

type console interface {
	Stdout() io.Writer
	Stderr() io.Writer
	Note() io.Writer
}

type redirectconsole struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *redirectconsole) Stdin() io.Reader {
	return c.stdin
}

func (c *redirectconsole) Stdout() io.Writer {
	return c.stdout
}

func (c *redirectconsole) Stderr() io.Writer {
	return c.stderr
}

func (c *redirectconsole) Note() io.Writer {
	return io.Discard
}

func getconsoleStdin(term console) io.Reader {
	if stdiner, ok := term.(interface{ Stdin() io.Reader }); ok {
		return stdiner.Stdin()
	}
	return os.Stdin
}

// expandWord evaluates a syntax.Word into a string using the full
// shell expansion pipeline (param expansion, tilde expansion, arithmetic,
// command substitution, etc.) via the expand package.
// Used by completions.go for tab completion.
func expandWord(w *syntax.Word) (string, error) {
	cfg := &expand.Config{
		Env: expand.ListEnviron(os.Environ()...),
		CmdSubst: func(io.Writer, *syntax.CmdSubst) error {
			return nil // skip command substitution, return empty
		},
		ProcSubst: func(*syntax.ProcSubst) (string, error) {
			return "", nil // skip process substitution, return empty
		},
	}
	return expand.Literal(cfg, w)
}

func formatStmt(source string, s *syntax.Stmt) string {
	return source[s.Pos().Offset():s.End().Offset()]
}

// runLine parses a shell line and executes it via the interp.Runner.
func runLine(runner *interp.Runner, term console, line string) error {
	parser := syntax.NewParser()
	var cmdErr error
	ctx := context.Background()

	// Catch SIGINT so hush survives when the user presses Ctrl+C during
	// an external command.  In cooked mode (after bubbletea exits / before
	// it starts), Ctrl+C sends SIGINT to the entire foreground process
	// group — both hush and the child.  This handler keeps hush alive.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	err := parser.Stmts(strings.NewReader(line), func(stmt *syntax.Stmt) bool {
		cmdErr = runner.Run(ctx, stmt)
		return cmdErr == nil
	})
	if err != nil {
		return err
	}
	if cmdErr != nil {
		return cmdErr
	}
	if parser.Incomplete() {
		return errors.New("Incomplete command: Multi-line commands not supported")
	}
	// Sync the OS working directory with interp's internal state, so that
	// os.Getwd() (used by the prompt and tab completion) reflects cd commands.
	if d := runner.Dir; d != "" {
		_ = os.Chdir(d)
	}
	return nil
}

// hushBuiltinMiddleware is an interp.ExecHandlers middleware that intercepts
// commands registered in the hush builtins map and executes them.
// It also syncs interp's exported env vars to os.Setenv before running
// hush builtins, so os.Environ() reflects shell exports.
// For non-builtin commands, it uses execHandlerNoExecBit which skips the
// executable mode bit check (unreliable on some filesystems like Wasm).
func hushBuiltinMiddleware(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return next(ctx, args)
		}
		name := args[0]
		if fn, ok := builtins[name]; ok {
			hc := interp.HandlerCtx(ctx)
			// Sync interp env to os so hush builtins (e.g. env) see exports.
			hc.Env.Each(func(name string, vr expand.Variable) bool {
				if vr.IsSet() && vr.Exported && vr.Kind == expand.String {
					os.Setenv(name, vr.Str)
				}
				return true
			})
			console := &interpConsole{hc: hc}
			err := fn(console, args[1:]...)
			if err != nil {
				var es interp.ExitStatus
				if !errors.As(err, &es) {
					// Print the error message, then wrap so the interp runner
					// treats it as a normal non-zero exit (not a fatal exit).
					fmt.Fprintln(hc.Stderr, err)
					return interp.ExitStatus(1)
				}
				return err
			}
			return nil
		}
		// Non-builtin: use no-exec-bit handler directly instead of next,
		// because the default handler checks file mode bits which are
		// unreliable on special filesystems (e.g. Wasm, FUSE).
		return execHandlerNoExecBit(2*time.Second)(ctx, args)
	}
}

// syncEnvHandler syncs interp's exported string environment variables
// to os.Setenv, so that hush code using os.Getenv (e.g. the env builtin)
// sees the same environment as the shell.
// Registered as a CallHandler, it fires on every command — after the
// previous command's side effects (like export) have been applied to
// interp's internal overlay.
func syncEnvHandler(ctx context.Context, args []string) ([]string, error) {
	hc := interp.HandlerCtx(ctx)
	hc.Env.Each(func(name string, vr expand.Variable) bool {
		if vr.IsSet() && vr.Exported && vr.Kind == expand.String {
			os.Setenv(name, vr.Str)
		}
		return true
	})
	return args, nil
}

// interpConsole adapts interp.HandlerContext to the hush console interface.
type interpConsole struct {
	hc interp.HandlerContext
}

func (c *interpConsole) Stdout() io.Writer { return c.hc.Stdout }
func (c *interpConsole) Stderr() io.Writer { return c.hc.Stderr }
func (c *interpConsole) Note() io.Writer   { return io.Discard }
func (c *interpConsole) Stdin() io.Reader  { return c.hc.Stdin }

// ctrlcAwareStdin wraps stdin to intercept Ctrl+C (byte 0x03) on Wasm,
// where there are no OS signals.  When 0x03 is detected, it cancels the
// context (which triggers the AfterFunc in execHandlerNoExecBit to kill
// the process via os.Kill), and filters the byte out of the stream.
// On non-Wasm platforms it's a no-op — the terminal driver handles SIGINT.
func ctrlcAwareStdin(cancel context.CancelFunc, stdin io.Reader) io.Reader {
	if runtime.GOARCH != "wasm" {
		return stdin
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stdin.Read(buf)
			if err != nil {
				return
			}
			// Scan for Ctrl+C and filter it out
			j := 0
			for i := 0; i < n; i++ {
				if buf[i] == 0x03 {
					cancel()
				} else {
					buf[j] = buf[i]
					j++
				}
			}
			if j > 0 {
				if _, err := pw.Write(buf[:j]); err != nil {
					return
				}
			}
		}
	}()
	return pr
}

// execHandlerNoExecBit returns an interp.ExecHandlerFunc that finds and executes
// binaries without checking the executable bit (mode bits are unreliable on
// some special filesystems like Wasm or FUSE).  Otherwise behaves like the
// default exec handler.
func execHandlerNoExecBit(killTimeout time.Duration) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		path, err := lookPathNoExecBit(hc.Dir, hc.Env, args[0])
		if err != nil {
			fmt.Fprintln(hc.Stderr, err)
			return interp.ExitStatus(127)
		}

		// Create a cancellable context so we can kill the process on Ctrl+C.
		// On Wasm this is triggered by ctrlcAwareStdin; on native by SIGINT.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		cmd := exec.Cmd{
			Path:   path,
			Args:   args,
			Env:    execEnvFromEnviron(hc.Env),
			Dir:    hc.Dir,
			Stdin:  ctrlcAwareStdin(cancel, hc.Stdin),
			Stdout: hc.Stdout,
			Stderr: hc.Stderr,
		}
		err = cmd.Start()
		if err == nil {
			stopf := context.AfterFunc(ctx, func() {
				if killTimeout <= 0 || runtime.GOOS == "windows" {
					_ = cmd.Process.Signal(os.Kill)
					return
				}
				_ = cmd.Process.Signal(os.Interrupt)
				time.Sleep(killTimeout)
				_ = cmd.Process.Signal(os.Kill)
			})
			defer stopf()
			err = cmd.Wait()
		}
		switch e := err.(type) {
		case *exec.ExitError:
			if status, ok := e.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return interp.ExitStatus(128 + status.Signal())
			}
			return interp.ExitStatus(e.ExitCode())
		case *exec.Error:
			fmt.Fprintf(hc.Stderr, "%v\n", e)
			return interp.ExitStatus(127)
		default:
			return e
		}
	}
}

// lookPathNoExecBit finds file in PATH but does NOT check the executable mode
// bit, unlike the interp's default lookPathDir/findExecutable/checkStat chain.
func lookPathNoExecBit(dir string, env expand.Environ, file string) (string, error) {
	if strings.ContainsAny(file, "/\\") {
		if !filepath.IsAbs(file) {
			file = filepath.Join(dir, file)
		}
		info, err := os.Stat(file)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("is a directory")
		}
		return file, nil
	}
	pathList := filepath.SplitList(env.Get("PATH").String())
	if len(pathList) == 0 {
		pathList = []string{""}
	}
	for _, elem := range pathList {
		var p string
		switch elem {
		case "", ".":
			p = "." + string(filepath.Separator) + file
		default:
			p = filepath.Join(elem, file)
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return p, nil
	}
	return "", fmt.Errorf("%q: executable file not found in $PATH", file)
}

// execEnvFromEnviron builds an os.Environ-style []string from an expand.Environ,
// replicating the interp's internal (unexported) execEnv helper.
func execEnvFromEnviron(env expand.Environ) []string {
	var list []string
	env.Each(func(name string, vr expand.Variable) bool {
		if !vr.IsSet() {
			return true
		}
		if vr.Exported && vr.Kind == expand.String {
			list = append(list, name+"="+vr.Str)
		}
		return true
	})
	return list
}
