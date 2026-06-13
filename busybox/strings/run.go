package strings

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const minLen = 4

// Run implements the strings command — extracts printable ASCII strings
// from binary files using byte-level scanning.
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
	br := bufio.NewReaderSize(r, 32768)
	var buf []byte
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %v", name, err)
		}
		if isPrint(b) {
			buf = append(buf, b)
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

// isPrint reports whether b is a printable ASCII character (0x20–0x7e) or tab.
func isPrint(b byte) bool {
	return b >= 0x20 && b <= 0x7e || b == '\t'
}
