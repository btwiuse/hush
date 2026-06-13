package strings

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unicode"
)

const minLen = 4

func Run(args []string) error {
	if len(args) < 2 || args[1] == "-" {
		return dump(os.Stdin, "<stdin>")
	}
	for _, path := range args[1:] {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %v\n", err)
			continue
		}
		err = dump(f, path)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func dump(r io.Reader, name string) error {
	br := bufio.NewReaderSize(r, 4096)
	var buf []rune
	for {
		ch, _, err := br.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %v", name, err)
		}
		if unicode.IsPrint(ch) || ch == '\t' {
			buf = append(buf, ch)
		} else {
			if len(buf) >= minLen {
				fmt.Println(string(buf))
			}
			buf = buf[:0]
		}
	}
	if len(buf) >= minLen {
		fmt.Println(string(buf))
	}
	return nil
}
