package main

import (
	"io"
	"log"
	"os"

	"github.com/btwiuse/hush"
)

func main() {
	log.SetOutput(io.Discard)
	os.Exit(hush.Run())
}
