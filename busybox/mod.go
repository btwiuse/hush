package busybox

import (
	"github.com/btwiuse/hush/busybox/ln"
	"github.com/btwiuse/hush/busybox/strings"
)

var Commands = map[string]func([]string) error{
	"ln":      ln.Run,
	"strings": strings.Run,
}
