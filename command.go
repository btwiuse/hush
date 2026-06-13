package hush

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
		return next(ctx, args)
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
