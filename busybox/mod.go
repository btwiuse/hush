package busybox

import (
	"github.com/btwiuse/hush/busybox/cat"
	"github.com/btwiuse/hush/busybox/head"
	"github.com/btwiuse/hush/busybox/ln"
	"github.com/btwiuse/hush/busybox/nl"
	"github.com/btwiuse/hush/busybox/strings"
	"github.com/btwiuse/hush/busybox/tail"
	"github.com/btwiuse/hush/busybox/timeout"
	"github.com/btwiuse/hush/busybox/ts"
	"github.com/btwiuse/hush/busybox/wc"
)

var Commands = map[string]func([]string) error{
	"cat":     cat.Run,
	"head":    head.Run,
	"ln":      ln.Run,
	"nl":      nl.Run,
	"strings": strings.Run,
	"tail":    tail.Run,
	"ts":      ts.Run,
	"wc":      wc.Run,
	"timeout": timeout.Run,
}
