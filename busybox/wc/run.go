package wc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

func Run(args []string) error {
	files := args[1:]
	if len(files) == 0 {
		lines, words, chars := count(os.Stdin)
		fmt.Printf("%8d%8d%8d\n", lines, words, chars)
		return nil
	}
	var totalLines, totalWords, totalChars int64
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %v\n", err)
			continue
		}
		lines, words, chars := count(f)
		f.Close()
		fmt.Printf("%8d%8d%8d %s\n", lines, words, chars, path)
		totalLines += lines
		totalWords += words
		totalChars += chars
	}
	if len(files) > 1 {
		fmt.Printf("%8d%8d%8d total\n", totalLines, totalWords, totalChars)
	}
	return nil
}

func count(r io.Reader) (lines, words, chars int64) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		lines++
		chars += int64(utf8.RuneCountInString(line) + 1) // +1 for newline
		words += int64(len(strings.Fields(line)))
	}
	return
}
