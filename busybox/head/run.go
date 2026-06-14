package head

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

func Run(args []string) error {
	set := flag.NewFlagSet("head", flag.ContinueOnError)
	n := set.Int("n", 10, "Number of lines")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}

	files := set.Args()
	if len(files) == 0 {
		return head(os.Stdin, *n)
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: %v\n", err)
			continue
		}
		if len(files) > 1 {
			fmt.Printf("==> %s <==\n", path)
		}
		head(f, *n)
		f.Close()
	}
	return nil
}

func head(r io.Reader, n int) error {
	sc := bufio.NewScanner(r)
	for i := 0; i < n && sc.Scan(); i++ {
		fmt.Println(sc.Text())
	}
	return sc.Err()
}
