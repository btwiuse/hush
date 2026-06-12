package hush

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Run runs the hush shell
func Run() int {
	return run(os.Stdin, os.Stdout, os.Stderr, os.Args)
}

func run(in io.Reader, out, outErr io.Writer, args []string) int {
	set := flag.NewFlagSet(args[0], flag.ContinueOnError)
	command := set.String("c", "", "Read and execute commands from the given string value.")
	err := set.Parse(args[1:])
	if err != nil {
		fmt.Fprintln(outErr, err)
		return 2
	}

	if *command != "" {
		reader := newRuneReader(strings.NewReader(*command))
		term := newTerminal(out, outErr)
		return term.ReadEvalPrintLoop(reader)
	}

	term := newTerminal(os.Stdout, os.Stderr)
	return term.bubblineReadEvalPrintLoop()
}
