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

### 被 interp 内置版本取代的：
`echo`, `cd`, `exit`, `pwd` — 这些仍在 builtins map 中但从未被调用（interp 内部优先处理），属死代码

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

### 1. Hush builtins 中存在死代码
`builtins` map 中包含 `echo`、`cd`、`exit`、`pwd`，但 interp 内部优先处理这些命令，hush 版本的实现永远不会被调用。

**影响**：无功能影响，只是代码冗余。

**建议**：可从 `init()` 的注册表中移除这四个函数，保留函数定义以防万一。

### 2. `evalWord` 重复实现
`command.go` 中保留了旧的 `evalWord` 函数，仅供 `completions.go` 的 tab 补全使用。它不支持 `$()`、`$((`、进程替换等扩展类型，遇到时会返回错误。

**影响**：补全功能受限，但不影响 interp 执行。

**建议**：可将补全改为使用 interp 的 expand 模块，或保持现状。

### 3. 死代码：`runForegroundExternal`
`tty_other.go:71` 和 `tty_js.go:18` 中的 `runForegroundExternal` 不再被调用（interp 处理外部命令执行）。

**影响**：无，只是代码垃圾。

### 4. 死代码：`ttySetup`
`tty_other.go:21` 中的 `ttySetup` 未被引用。

### 5. 死代码：`newRuneReader`
`rune_reader.go` 中的 `newRuneReader` 未被引用（旧 REPL 使用，已被 bubbleline 取代）。

### 6. 死代码：旧的 ReadEvalPrintLoop
`terminal.go:88` 的 `ReadEvalPrintLoop` 和对应的 escape-sequence REPL 代码不再作为入口使用（bubbleline 是主 REPL）。但方法本身仍被维护且可工作。

**影响**：代码可读性降低，维护成本增加。

### 7. `export` 环境变量隔离
interp 通过内部 `writeEnv` 管理环境变量，不调用 `os.Setenv()`。因此 `export VAR=val` 后，`os.Getenv("VAR")` 看不到该变量。

**影响**：在 REPL 中不影响功能（interp 会将正确环境传给子进程）。但如果外部代码依赖 `os.Getenv` 查看 shell 设置的变量，会失败。

### 8. `exit` 不再打印退出信息
旧的 hush `exit` 会打印 `Exited with code X`（红色）。interp 的 `exit` 不打印任何信息。

**影响**：用户习惯的变化。

### 9. `echo` 行为变化
旧的 hush `echo` 只是简单的 `strings.Join(args, " ")`。interp 的 `echo` 支持 `-n` 和转义序列。

**影响**：行为更标准，但可能与旧脚本略有不同。

### 10. Wasm 兼容性未验证
`github.com/btwiuse/sh/v3` fork 声称支持 Wasm，但未在 `GOOS=js GOARCH=wasm` 下测试。

**影响**：可能无法正常工作。

### 11. 提示符中的 `lastExitCode` 可能漂移
hush 的 `terminal.lastExitCode` 和 interp 的 `runner.lastExit` 都跟踪退出码。目前 REPL 手动同步两者，但存在不同步的风险。

**建议**：REPL 可以直接从 runner 读取退出码，消除手动跟踪。

### 12. 无 rc file 支持
没有 `~/.hushrc` 启动脚本机制（与迁移前一致）。

## 总结

迁移是成功的：build、vet、test 全部通过，smoke test 正常。核心改动集中在 3 个文件（`command.go`、`hush.go`、`repl.go`），TUI 层零改动。获得了完整的 POSIX/Bash shell 语义支持，代价是约 12 个小缺陷/死代码问题，主要集中在冗余代码和细微行为差异上。
