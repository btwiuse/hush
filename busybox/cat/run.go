package cat

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func Run(args []string) error {
	files := args[1:]
	if len(files) == 0 {
		return cat(os.Stdin)
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cat: %v\n", err)
			continue
		}
		err = cat(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func cat(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	return sc.Err()
}
