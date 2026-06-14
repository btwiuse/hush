package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/btwiuse/hush"
	"github.com/btwiuse/sh/v3/interp"
	"github.com/btwiuse/sh/v3/syntax"
	"golang.org/x/term"
)

func main() {
	command := flag.String("c", "", "command to be executed")
	flag.Parse()

	runner := hush.NewRunner(&hush.Console{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})

	err := runAll(runner, *command)
	var es interp.ExitStatus
	if errors.As(err, &es) {
		os.Exit(int(es))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAll(runner *interp.Runner, command string) error {
	if command != "" {
		return runScript(runner, strings.NewReader(command), "")
	}
	if flag.NArg() > 0 {
		for _, path := range flag.Args() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			err = runScript(runner, f, path)
			f.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return runInteractive(runner, os.Stdin, os.Stdout)
	}
	return runPipe(runner, os.Stdin)
}

func runScript(runner *interp.Runner, reader io.Reader, name string) error {
	prog, err := syntax.NewParser().Parse(reader, name)
	if err != nil {
		return err
	}
	runner.Reset()
	ctx := context.Background()
	return runner.Run(ctx, prog)
}

func runInteractive(runner *interp.Runner, stdin io.Reader, stdout io.Writer) error {
	parser := syntax.NewParser()
	fmt.Fprintf(stdout, "$ ")
	for stmts, err := range parser.InteractiveSeq(stdin) {
		if err != nil {
			return err
		}
		if parser.Incomplete() {
			fmt.Fprintf(stdout, "> ")
			continue
		}
		ctx := context.Background()
		for _, stmt := range stmts {
			if err := runner.Run(ctx, stmt); err != nil {
				if runner.Exited() {
					return err
				}
				fmt.Fprintln(os.Stderr, err)
			}
		}
		fmt.Fprintf(stdout, "$ ")
	}
	return nil
}

func runPipe(runner *interp.Runner, stdin io.Reader) error {
	parser := syntax.NewParser()
	for stmts, err := range parser.InteractiveSeq(stdin) {
		if err != nil {
			return err
		}
		if parser.Incomplete() {
			continue
		}
		ctx := context.Background()
		for _, stmt := range stmts {
			if err := runner.Run(ctx, stmt); err != nil {
				if runner.Exited() {
					return err
				}
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	return nil
}
