package nl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func Run(args []string) error {
	r := io.Reader(os.Stdin)
	if len(args) > 1 {
		f, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	return nl(r)
}

func nl(r io.Reader) error {
	sc := bufio.NewScanner(r)
	n := 1
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			fmt.Println()
		} else {
			fmt.Printf("%6d\t%s\n", n, line)
			n++
		}
	}
	return sc.Err()
}
