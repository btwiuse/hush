package cat

import (
	"fmt"
	"io"
	"os"
)

func Run(args []string) error {
	files := args[1:]
	if len(files) == 0 {
		_, err := io.Copy(os.Stdout, os.Stdin)
		return err
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cat: %v\n", err)
			continue
		}
		_, err = io.Copy(os.Stdout, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
