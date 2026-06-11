package hush

import (
	"io"
)

type carriageReturnWriter struct {
	io.Writer
}

func newCarriageReturnWriter(dest io.Writer) (io.Writer, error) {
	return &carriageReturnWriter{dest}, nil
}

func (c *carriageReturnWriter) Write(buf []byte) (n int, err error) {
	out := make([]byte, 0, len(buf))
	for _, b := range buf {
		if b == '\n' {
			out = append(out, '\n', '\r')
		} else {
			out = append(out, b)
		}
	}
	_, err = c.Writer.Write(out)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}
