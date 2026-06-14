package hush

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/term"
)

func cat(term console, args ...string) error {
	if len(args) == 0 {
		return catStdin(term)
	}

	for _, path := range args {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return errors.Errorf("%s: Is a directory", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(term.Stdout(), f)
		if err != nil {
			return err
		}
	}
	return nil
}

func catStdin(term console) error {
	println("catting!!!")
	stdin := getconsoleStdin(term)
	stdout := term.Stdout()
	stdoutIsTerm := isTerminal(stdout)

	buf := make([]byte, 1)
	for {
		println("stdin.Read byte")
		n, err := stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if n == 0 {
			continue
		}
		b := buf[0]
		switch b {
		case '\x04': // Ctrl+D
			return nil
		case '\x03': // Ctrl+C
			return nil
		case '\r': // Enter in raw mode
			stdout.Write([]byte{'\n'})
			if !stdoutIsTerm {
				term.Stderr().Write([]byte{'\n'})
			}
		default:
			stdout.Write([]byte{b})
			if !stdoutIsTerm {
				term.Stderr().Write([]byte{b})
			}
		}
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
