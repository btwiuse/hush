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
	start := 0
	for i, b := range buf {
		if b == '\n' {
			// write everything up to and including \n
			_, err = c.Writer.Write(buf[start : i+1])
			if err != nil {
				return n, err
			}
			// then insert \r after \n
			_, err = c.Writer.Write([]byte{'\r'})
			if err != nil {
				return i + 1, err
			}
			start = i + 1
		}
		n++
	}
	// flush trailing bytes (no \n in this segment)
	if start < len(buf) {
		_, err = c.Writer.Write(buf[start:])
	}
	return
}
