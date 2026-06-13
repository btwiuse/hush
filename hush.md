# hush — 架构解耦记录

## 一开始

`./cmd/hush/main.go` → `package hush` 承担了两件事：

1. **Shell** — 命令解析、执行、内置命令、busybox
2. **rlwrap** — 行编辑（bubbline）、历史、补全、终端 raw mode、光标管理

两者完全耦合在同一个进程里：

```
hush Run()
  └─ run()
       ├─ -c / 脚本文件 → runner.Run() 直接执行
       └─ REPL →
            newTerminal()
            └─ bubblineReadEvalPrintLoop()
                 ├─ bubbline 行编辑、历史、补全
                 ├─ updatePrompt() 渲染 ➜ dir $ 提示
                 └─ runLine() → runner.Run() 执行
```

所有代码在同一个 `package hush` 里，`terminal.go`、`repl.go`、`prompt.go`、`completions.go` 互相引用，无法独立使用。

## 交付成果（拆分后）

两个独立 binary，通过 stdin pipe 组合：

```
hu (智能行编程序)       sh (裸 REPL)
─────────────────       ──────────
bubbline 行编辑          parser.InteractiveSeq
历史、补全                $ / > 提示（终端模式）
multi-line               无提示（pipe 模式）
智能提示检测                │
      │                    │
      │   PTY (master)     │
      ├───────────────────►│  stdin
      │◄───────────────────┤  stdout (经 ptyOutput)
      │   检测最后一行      │
      │   识别 prompt      │
      │   用 child 的真实  │
      │   提示符显示        │
```

### cmd/sh

参考 [gosh](https://github.com/btwiuse/sh/tree/master/cmd/gosh) 实现，三种模式：

| 模式 | 触发条件 | 行为 |
|------|---------|------|
| `-c` | `-c 'cmd'` | 解析执行后退出 |
| 脚本 | 文件名参数 | 依次执行脚本后退出 |
| 终端 | `term.IsTerminal(stdin)` | `InteractiveSeq` + `$` / `>` 提示 |
| Pipe | stdin 是 pipe | `InteractiveSeq` 静默执行，无提示 |

### cmd/hu

基于 bubbline 的智能行编程序，通过 PTY 连接子进程，功能：

- 行编辑（Emacs 键绑定）
- 历史记录（`~/.history`，自动保存）
- 多行输入（`\` 续行）
- Ctrl+D / Ctrl+C 处理
- **自动提示检测**：读取子进程 PTY 输出，跟踪最后一个不完整行作为提示符
- **智能 GetLine**：仅在检测到子进程处于提示符等待状态时才显示输入区

#### 组合方式

```sh
# 等同于原来 ./cmd/hush
./hu ./sh

# hu 可以搭配任意 stdin 驱动的程序使用
./hu python3
./hu lua
./hu bc
```

#### 架构

```
ptyOutput (后台 goroutine)
  ├─ ptmx.Read() → 读取子进程所有输出
  ├─ os.Stdout.Write() → 直通终端
  └─ 跟踪 lastLine（最后一个不完整行）
       └─ WaitForPrompt(idleTimeout)
            └─ 输出静默 100ms 后返回 lastLine 作为 prompt

runEditor 主循环:
  1. WaitForPrompt → 拿到子进程真实 prompt
  2. m.Prompt = detectedPrompt
  3. m.GetLine() → 用户输入
  4. ptmx.Write(input) → 发送给子进程
  5. 回到 1
```

#### 文件结构

| 文件 | 标签 | 职责 |
|------|------|------|
| `main.go` | 无 | 共享辅助函数（补全、续行检测、fallback 提示） |
| `hu_native.go` | `!js` | `main()` + prompt 感知的 `runEditor` 循环 |
| `hu_js.go` | `js` | `main()` + 简单 `runEditor`（wasm 回退） |
| `pty_native.go` | `!js` | `newCmd()` 返回 `*os.File` + `ptyOutput` 提示检测器 |
| `pty_js.go` | `js` | `newCmd()` 返回 `io.WriteCloser` |

### package hush 新增公开 API

```go
func NewRunner(in, out, outErr) *interp.Runner  // 创建 runner
func RunLine(runner, line) error                 // 执行单行
```

`cmd/sh` 和旧的 `cmd/hush` 共享同一套执行逻辑。

## 不足之处

### ~~1. 无 PTY，信号处理不完整~~

已解决。使用 `github.com/creack/pty` 通过 PTY 连接 `hu` 和 `sh`，sh 获得真正的终端。

原问题描述：

- **Ctrl+C 竞争**：bubbletea 安装了自己的 SIGINT handler。当用户按 Ctrl+C 时，SIGINT 发往前台进程组（hu + sh + 子进程）。bubbletea 捕获信号处理，`sh` 和子进程也收到信号。多数情况下行为正确，但在时序窗口内 bubbletea 可能先处理信号，干扰 `sh` 的执行。
- **无作业控制**：`sh` 在 pipe 模式下运行，`interp` 的作业控制不可用。
- **无法检测 `sh` 的退出**：`hu` 通过 write EPIPE 被动发现 `sh` 已退出，而非主动 wait。

### ~~2. 提示符不完整~~

已解决。`hu` 自动检测子进程（sh/python/lua 等）输出的真实提示符。

工作原理：`ptyOutput` 后台读取 PTY 输出，跟踪最后一个不完整行（即子进程停留在提示符等待输入时的最后一行）。当输出静默 100ms 后，将该行作为 `bubbline` 的 prompt 显示。

若检测失败（子进程无提示或退出），回退到绿色 `➜ dir $`。

### ~~3. Tab 补全未实现~~

已解决。`hu` 的 `hushAutoComplete` 实现了基于文件系统的路径补全：找到光标所在单词，按 `/` 分割目录和前缀，列出匹配项。目录带 `/` 后缀。

### ~~4. 输出与 bubbline 显示交错~~

已解决。新架构移除了盲目的 `io.Copy(os.Stdout, ptmx)`，改为 `ptyOutput` 后台 goroutine 管理 PTY 输出：

- **子进程输出时**：所有输出通过 goroutine 直通 `os.Stdout`，此时 `GetLine` 不在运行，无交错问题。
- **等待输入时**：`WaitForPrompt` 检测到输出静默 100ms 后才调用 `GetLine`，此时子进程无输出，bubbline 独占终端。
- **切换时机**：按下回车后立即退出 `GetLine`，进入 passthrough 模式；检测到 prompt 后再重新进入输入模式。

### 5. 多行构造无视觉提示

`checkInputComplete` 只识别反斜杠续行（`\`），不识别 shell 关键字：

```sh
# 以下场景在 sh 侧可以正确跨行执行
if true           # ← hu 立即提交给 sh
then echo ok      # ← hu 提交，sh 仍等待 fi
fi                # ← sh 终于完成并执行
```

但用户看不到 `>` 续行提示。bubbline 每次显示的都是主提示符，用户可能误认为上一个命令已执行完毕。

### 6. Windows / js/wasm 支持

PTY 通过 build tag 分离：native 使用 `creack/pty`，js/wasm 回退到 `os.Pipe()`。
