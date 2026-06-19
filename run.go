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

// Run runs the hush shell
// NewRunner creates an interp.Runner with all hush builtins and middleware.
func NewRunner(term *Console) *interp.Runner {
	runner, err := interp.New(
		interp.StdIO(term.Stdin, term.Stdout, term.Stderr),
		interp.Interactive(true),
		interp.ExecHandlers(hushBuiltinMiddleware),
		interp.CallHandler(syncEnvHandler),
	)
	if err != nil {
		panic(err)
	}
	return runner
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
	rcfile := set.String("rcfile", "", "Source RC file on startup (default ~/.profile)")
	err := set.Parse(args[1:])
	if err != nil {
		fmt.Fprintln(outErr, err)
		return 2
	}

	runner := NewRunner(&Console{Stdin: in, Stdout: out, Stderr: outErr})

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
	sourceRCFile(runner, *rcfile, outErr)
	r := newRepl(&Console{Stdin: in, Stdout: out, Stderr: outErr}, runner)
	return r.bubblineReadEvalPrintLoop()
}

func sourceRCFile(runner *interp.Runner, rcfile string, stderr io.Writer) {
	if rcfile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		rcfile = filepath.Join(home, ".profile")
	}
	f, err := os.Open(rcfile)
	if err != nil {
		return
	}
	defer f.Close()
	parser := syntax.NewParser()
	prog, err := parser.Parse(f, rcfile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return
	}
	runner.Reset()
	if err := runner.Run(context.Background(), prog); err != nil {
		fmt.Fprintln(stderr, err)
	}
}
