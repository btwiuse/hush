package ts

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
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
	return ts(r)
}

func ts(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		now := time.Now().Format("2006-01-02T15:04:05")
		fmt.Printf("%s %s\n", now, sc.Text())
	}
	return sc.Err()
}
