package hush

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/btwiuse/hush/busybox"
	"github.com/btwiuse/sh/v3/interp"
	"github.com/btwiuse/sh/v3/syntax"
)

func errcode(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

// Run runs the hush shell
// NewRunner creates an interp.Runner with all hush builtins and middleware.
func NewRunner(in io.Reader, out, outErr io.Writer) *interp.Runner {
	runner, err := interp.New(
		interp.StdIO(in, out, outErr),
		interp.Interactive(true),
		interp.ExecHandlers(hushBuiltinMiddleware),
		interp.CallHandler(syncEnvHandler),
	)
	if err != nil {
		panic(err)
	}
	return runner
}

// RunLine executes a single shell line. It is a public wrapper around runLine.
func RunLine(runner *interp.Runner, line string) error {
	return runLine(runner, nil, line)
}

func Run() int {
	if cmd, ok := busybox.Commands[filepath.Base(os.Args[0])]; ok {
		return autoerr(cmd(os.Args))
	}
	return run(os.Stdin, os.Stdout, os.Stderr, os.Args)
}

func autoerr(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func run(in io.Reader, out, outErr io.Writer, args []string) int {
	set := flag.NewFlagSet(args[0], flag.ContinueOnError)
	command := set.String("c", "", "Read and execute commands from the given string value.")
	err := set.Parse(args[1:])
	if err != nil {
		fmt.Fprintln(outErr, err)
		return 2
	}

	runner := NewRunner(in, out, outErr)

	if *command != "" {
		reader := strings.NewReader(*command)
		parser := syntax.NewParser()
		ctx := context.Background()
		prog, err := parser.Parse(reader, "")
		if err != nil {
			fmt.Fprintln(outErr, err)
			return 2
		}
		if err := runner.Run(ctx, prog); err != nil {
			var es interp.ExitStatus
			if errors.As(err, &es) {
				return int(es)
			}
			fmt.Fprintln(outErr, err)
			return 1
		}
		return 0
	}

	// Script files
	if set.NArg() > 0 {
		ctx := context.Background()
		parser := syntax.NewParser()
		for _, path := range set.Args() {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintln(outErr, err)
				return 1
			}
			prog, err := parser.Parse(f, path)
			f.Close()
			if err != nil {
				fmt.Fprintln(outErr, err)
				return 2
			}
			runner.Reset()
			if err := runner.Run(ctx, prog); err != nil {
				var es interp.ExitStatus
				if errors.As(err, &es) {
					return int(es)
				}
				fmt.Fprintln(outErr, err)
				return 1
			}
		}
		return 0
	}

	// REPL mode
	term := newTerminal(out, outErr, runner)
	return term.bubblineReadEvalPrintLoop()
}
