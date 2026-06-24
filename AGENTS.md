# hush — Agent Guide

A simple Bourne-like shell written in Go, compatible with Wasm.

## Quick Start

```sh
make            # lint + test
make lint       # golangci-lint run (native + js/wasm)
make test       # go test -race -coverprofile=cover.out ./...
go build ./cmd/hush
```

## Architecture

```
hush.Run()             # entry point (cmd/hush/main.go)
  → run()              # flag parsing, tty setup
    → newTerminal()
      → terminal.bubblineReadEvalPrintLoop()
        → runLine()  # parse with mvdan.cc/sh/v3, dispatch
          → runCommand()
            → runCallExpr()  # external process or builtin
            → runCmd()       # route: builtin (env-override) or exec.Cmd
```

## Key Files

| File | Purpose |
|------|---------|
| `run.go` | Public `Run()`/`NewRunner()` API, `-c` flag, REPL boot |
| `repl.go` | REPL loop via bubbles (bubblineReadEvalPrintLoop) |
| `terminal.go` | Terminal state, escape/control handling, cursor mgmt |
| `command.go` | Shell parsing, `runLine`/`runCommand`, word eval, builtin dispatch |
| `builtins.go` | 23 builtins: cat, cd, chmod, clear, curl, echo, env, exit, ln, ls, mkdir, mv, pwd, rm, rmdir, touch, which |
| `cat.go` | `cat` builtin implementation |
| `prompt.go` | Go template prompt: `{{.RCArrow}} {{.CurDirName}} $ ` |
| `completions.go` | Tab completion — file paths only (when word contains `/`) |
| `js_builtins.go` | Wasm-only builtins: `jseval`, `jsdownload` |
| `js_download.go` | Browser file download via Blob/URL API |

## Key Types

- **`repl`** — REPL state: holds `*Console` and `*interp.Runner`, has `bubblineReadEvalPrintLoop`
- **`Console`** — struct with public `Stdin`, `Stdout`, `Stderr` fields (used everywhere for output)
- **`builtinFunc`** — `func(term Console, args ...string) error`

## Platform Separation (Build Tags)

Uses `//go:build js` constraints.

- **js-tagged**: `js_builtins.go`, `js_download.go` — Wasm/js compatibility
- **non-js-tagged**: All other files — native builds

Wasm-specific builtins (`jseval`, `jsdownload`) register themselves in `js_builtins.go`'s `init()`.

Wasm mode requires a [forked Go compiler](https://github.com/btwiuse/go) with Node.js process management patches.

## Supported Shell Features

**Yes**: `&&`, `||`, `|`, `|&`, `time`, `~` tilde expansion, `$VAR` param expansion, redirects (`>`, `>>`, `&>`, `&>>`, `<`, `<<`, `<<-`, `<<<`), background (`&`), heredocs, `for`, `while`, `case`, backtick cmd substitution.

**Not supported**: `{block}`, `(subshell)`, `func()`, arithmetic commands, test clauses, declare/let/coproc, `$()` cmd subst, arithmetic expansion, proc subst, extglob, slicing/replacement in param expansion.

## Naming & Style

- One file per concern (no splitting across packages — everything in package `hush`)
- Builtins: lowercase function names (`echo`, `ls`, etc.)
- `ioutil` is used throughout despite being deprecated since Go 1.16 (lots of LSP hints)
- `parser.Stmts`/`parser.Words` deprecated APIs used vs `StmtsSeq`/`WordsSeq`
- Color output via `github.com/fatih/color`
- Errors via `github.com/pkg/errors` (wrapping with `errors.Wrap`, formatting with `errors.Errorf`)

## Testing

- Single test file: `hush_test.go`
- Table-driven with `t.Parallel()` (copies loop var `tc`)
- Tests the internal `run()` function (not `Run()`)
- Passes `bytes.Buffer` for I/O (no real terminal needed)
- Prompt is verified via `color.GreenString("➜")` (depends on color package)
- Also runs under `GOOS=js GOARCH=wasm` in CI

## REPL Implementation

hush uses [bubbline](https://github.com/justwasm/bubbline) (built on [Bubble Tea](https://github.com/charmbracelet/bubbletea)) for line editing — this handles history, cursor movement, Ctrl shortcuts, Tab completion, and multi-line input.

- **History file**: `~/.history`, app-only, managed by bubbles (no manual rotation needed).

## Dependencies

| Package | Purpose |
|---------|---------|
| `mvdan.cc/sh/v3` | Shell syntax parser |
| `github.com/fatih/color` | Colored terminal output |
| `golang.org/x/term` | Raw TTY mode for `cat` and `cmd/` tools |
| `github.com/johnstarich/go/datasize` | Byte formatting for `ls -l` |
| `github.com/pkg/errors` | Error wrapping and formatting |
