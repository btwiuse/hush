package tail

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

func Run(args []string) error {
	set := flag.NewFlagSet("tail", flag.ContinueOnError)
	n := set.Int("n", 10, "Number of lines")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}

	files := set.Args()
	if len(files) == 0 {
		return tail(os.Stdin, *n, "<stdin>")
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail: %v\n", err)
			continue
		}
		if len(files) > 1 {
			fmt.Printf("==> %s <==\n", path)
		}
		tail(f, *n, path)
		f.Close()
	}
	return nil
}

func tail(r io.Reader, n int, name string) error {
	sc := bufio.NewScanner(r)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if sc.Err() != nil {
		return fmt.Errorf("%s: %v", name, sc.Err())
	}
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	for _, l := range lines[start:] {
		fmt.Println(l)
	}
	return nil
}
