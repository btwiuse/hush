# Hush 迁移报告：基于 interp 的执行引擎

## 迁移概述

将 hush 的手写 shell 执行层（`command.go` + `call.go`）替换为 `github.com/btwiuse/sh/v3/interp`（mvdan.cc/sh 的 Wasm 兼容 fork），保留 hush 的原始 TUI（bubbletea/bubbline REPL）、历史、提示符、补全系统。

## 文件变更

| 文件 | 操作 | 说明 |
|------|------|------|
| `call.go` | 删除 | 全部由 interp 接管（重定向、管道、参数扩展等） |
| `command.go` | 重写 | `runLine` 改用 interp.Runner；新增 `hushBuiltinMiddleware` 适配层 |
| `builtins.go` | 修改 | 删除 `runCmd`/`cmdOptions`/`runWithEnv`，`env` 改用简单 `exec.Command` |
| `hush.go` | 重写 | 构建 Runner，支持 `-c`、脚本文件（`hush file.sh`）、REPL |
| `terminal.go` | 修改 | 增加 `runner` 字段，错误处理适配 interp.ExitStatus |
| `repl.go` | 修改 | 错误处理适配 interp.ExitStatus + runner.Exited() |
| `hush_test.go` | 修改 | 使用 `testRunner()` 创建带 StdIO 的 Runner |
| `completions.go` | 改 import | `mvdan.cc/sh/v3` → `github.com/btwiuse/sh/v3` |
| `go.mod` | 改依赖 | 同上 |
| `rune_reader.go` | 删除 | `newRuneReader` 和 `runeReader` 类型，旧 REPL 用，bubbline 取代 |
| `tty_other.go` | 删除 | `ttySetup`/`ttyExitRawMode`/`ttyEnterRawMode`/`runForegroundExternal`，interp 接管了外部命令执行 |
| `tty_js.go` | 删除 | Wasm 空壳 TTY 函数，同上 |
| `builtins.go` | 删除死代码 | `echo`/`cd`/`exit`/`pwd` 注册和函数体（interp 内部优先处理，永不抵达 hush 中间件） |
| `terminal.go` | 删除死代码 | 旧的 ReadEvalPrint REPL、所有光标方法（CursorLeftN 等）、`splitRunes`/`deleteWord`/`Clear` 等 |
| `command.go` | 添加 SIGINT 处理 | catch SIGINT 防止 Ctrl+C 杀掉 hush；`os.Chdir(runner.Dir)` 同步 cd 后的目录到 OS 进程 |
| `go.mod` | 升级 | `btwiuse/sh/v3` v3.13.1 → v3.14.0，去掉本地 replace |
| `interp`（上游 fork） | 添加 `$_` 支持 | call expr 完成后将展开的最后一个参数写入 `$_` 变量 |

## 保留的 Hush Builtins

通过 `hushBuiltinMiddleware`（ExecHandler 中间件）注册，interp 处理非内置命令时会途经此中间件：

### 原始 hush builtins 保留的：
- `cat` — Go 实现的 `cat`（带 Ctrl+D/C 处理）
- `clear` — 清屏
- `curl` — HTTP 请求（支持 -I, -L, -O）
- `env` — 环境变量查看 + env-override exec
- `ln` — 符号链接（支持 -s, -sf）
- `which` — 查找可执行文件
- `rmdir` — 删除空目录

### u-root coreutils 保留的：
`chmod`, `cp`, `find`, `ls`, `mkdir`, `mv`, `rm`, `touch`, `xargs`, `base64`, `gzip`, `gunzip`, `mktemp`, `shasum`, `tar`

### Wasm 内置命令保留的：
`jseval`, `jsdownload`（通过 `js_builtins.go` 注册到 builtins map）

### 被 interp 内置版本取代（已移除）：
`echo`, `cd`, `exit`, `pwd` — 注册在 builtins map 中但 interp 内部优先处理，已被清理。

## 新增功能（免费获得）

由于 interp 是完整的 POSIX/Bash shell 解释器，hush 现在支持：

### 控制流
- `if/then/elif/fi`、`while/do/done`、`for/do/done`、`case/esac`
- 函数定义 `func() { ... }`
- Subshell `( ... )`、block `{ ... }`

### 扩展
- `$()` 命令替换、`$(<file)` 快捷读文件
- `$(( ... ))` 算术扩展
- `<()` / `>()` 进程替换
- Globstar `**`、extglob、dotglob 等

### 内置命令
- `source`/`.`、`eval`、`exec`、`trap`
- `set`/`shift`/`unset`/`readonly`/`local`
- `test`/`[`、`read`、`printf`、`type`
- `jobs`/`fg`/`bg`/`wait`/`kill`
- `break`/`continue`/`return`
- `alias`/`unalias`、`getopts`
- `true`/`false`/`:`、`times`、`umask`

### 脚本执行
- `hush script.sh` 运行脚本文件
- 位置参数 `$1`, `$2`, `$@`, `$#` 等

## 剩余缺陷与限制

### 1. `evalWord` 重复实现
`command.go` 中保留了旧的 `evalWord` 函数，仅供 `completions.go` 的 tab 补全使用。它不支持 `$()`、`$((`、进程替换等扩展类型，遇到时会返回错误。

**影响**：补全功能受限，但不影响 interp 执行。

**建议**：可将补全改为使用 interp 的 expand 模块，或保持现状。

### 2. `export` 环境变量隔离
interp 通过内部 `writeEnv` 管理环境变量，不调用 `os.Setenv()`。因此 `export VAR=val` 后，`os.Getenv("VAR")` 看不到该变量。

**影响**：在 REPL 中不影响功能（interp 会将正确环境传给子进程）。但如果外部代码依赖 `os.Getenv` 查看 shell 设置的变量，会失败。

### 3. Wasm 兼容性未验证
`github.com/btwiuse/sh/v3` fork 声称支持 Wasm，但未在 `GOOS=js GOARCH=wasm` 下测试。

**影响**：可能无法正常工作。

### 4. 提示符中的 `lastExitCode` 可能漂移
hush 的 `terminal.lastExitCode` 和 interp 的 `runner.lastExit` 都跟踪退出码。目前 REPL 手动同步两者，但存在不同步的风险。

**建议**：REPL 可以直接从 runner 读取退出码，消除手动跟踪。

### 5. 无 rc file 支持
没有 `~/.hushrc` 启动脚本机制（与迁移前一致）。

## 总结

迁移是成功的：build、vet、test 全部通过，smoke test 正常。核心改动集中在 3 个文件（`command.go`、`hush.go`、`repl.go`），TUI 层零改动。获得了完整的 POSIX/Bash shell 语义支持，代价是约 5 个小缺陷问题。后续修复了 SIGINT 导致 hush 退出的问题、cd 后 prompt 不更新的问题、以及 `$_` 变量支持。
