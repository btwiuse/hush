package hush

import (
	"context"
	"io"
	"os"
	"strings"

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

// evalWord evaluates syntax.WordParts into a string.
// Kept for use by completions.go.
func evalWord(parts []syntax.WordPart) (string, error) {
	s := ""
	for ix, part := range parts {
		switch part := part.(type) {
		case *syntax.Lit:
			s += part.Value
			if ix == 0 && (s == "~" || strings.HasPrefix(s, "~/")) {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				s = homeDir + s[1:]
			}
		case *syntax.SglQuoted:
			if part.Dollar {
				return "", errors.Errorf("Dollar single-quotes not supported: %v", part)
			}
			s += part.Value
		case *syntax.DblQuoted:
			if part.Dollar {
				return "", errors.Errorf("Dollar single-quotes not supported: %v", part)
			}
			dblQuoted, err := evalWord(part.Parts)
			if err != nil {
				return "", err
			}
			s += dblQuoted
		case *syntax.ParamExp:
			name := part.Param.Value
			if part.Excl || part.Length || part.Width || part.Index != nil || part.Slice != nil || part.Repl != nil || part.Names != 0 || part.Exp != nil {
				return "", errors.Errorf("Variable expansion type not supported: %s %v", name, part)
			}
			s += os.Getenv(name)
		case *syntax.CmdSubst, *syntax.ArithmExp, *syntax.ProcSubst, *syntax.ExtGlob:
			return "", errors.Errorf("Unrecognized word part type: %T %v", part, part)
		default:
			return "", errors.Errorf("Unrecognized word part type: %T %v", part, part)
		}
	}
	return s, nil
}

func formatStmt(source string, s *syntax.Stmt) string {
	return source[s.Pos().Offset():s.End().Offset()]
}

// runLine parses a shell line and executes it via the interp.Runner.
func runLine(runner *interp.Runner, term console, line string) error {
	parser := syntax.NewParser()
	var cmdErr error
	ctx := context.Background()

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
	return nil
}

// hushBuiltinMiddleware is an interp.ExecHandlers middleware that intercepts
// commands registered in the hush builtins map and executes them.
func hushBuiltinMiddleware(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return next(ctx, args)
		}
		name := args[0]
		if fn, ok := builtins[name]; ok {
			hc := interp.HandlerCtx(ctx)
			console := &interpConsole{hc: hc}
			err := fn(console, args[1:]...)
			if err != nil {
				return err
			}
			return nil
		}
		return next(ctx, args)
	}
}

// interpConsole adapts interp.HandlerContext to the hush console interface.
type interpConsole struct {
	hc interp.HandlerContext
}

func (c *interpConsole) Stdout() io.Writer { return c.hc.Stdout }
func (c *interpConsole) Stderr() io.Writer { return c.hc.Stderr }
func (c *interpConsole) Note() io.Writer   { return io.Discard }
func (c *interpConsole) Stdin() io.Reader  { return c.hc.Stdin }
