# hush  [![CI](https://github.com/btwiuse/hush/actions/workflows/ci.yml/badge.svg)](https://github.com/btwiuse/hush/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/btwiuse/hush/badge.svg?branch=main)](https://coveralls.io/github/btwiuse/hush?branch=main)

A simple Bourne-like shell, compatible with Wasm. Written in Go.

## Getting started

Install like any other go library:
```
go install github.com/btwiuse/hush/cmd/hush@latest
```

Alternatively, import `hush` in your own projects:
```go
package main

import (
    "os"
    "github.com/btwiuse/hush"
)

func main() {
    exitCode := hush.Run()
    os.Exit(exitCode)
}
```

## Wasm compatibility

Today, Go's Web Assembly support does not include running processes.
To make this possible, Hush is built with [hackpad's `go` fork](https://github.com/btwiuse/go). The fork contains patches that spawn and manage processes using Node.js's APIs, similar to the Node.js file system API used upstream.
