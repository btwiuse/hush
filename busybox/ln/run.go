package ln

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func Run(args []string) error {
	set := flag.NewFlagSet("ln", flag.ContinueOnError)
	symbolic := set.Bool("s", false, "Create symbolic link")
	force := set.Bool("f", false, "Remove existing destination file")

	var expanded []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") && len(a) > 2 && a[0] == '-' && a[1] != '-' {
			for _, ch := range a[1:] {
				expanded = append(expanded, "-"+string(ch))
			}
		} else {
			expanded = append(expanded, a)
		}
	}

	if err := set.Parse(expanded); err != nil {
		return err
	}

	if !*symbolic {
		return errors.New("Only -s (symbolic) links are supported")
	}

	if set.NArg() != 2 {
		return fmt.Errorf("Not enough args")
	}

	target := set.Arg(0)
	linkName := set.Arg(1)

	if *force {
		if err := os.Remove(linkName); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return os.Symlink(target, linkName)
}
