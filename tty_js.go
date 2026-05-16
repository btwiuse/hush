//go:build js

package hush

import "context"

func ttySetup() (context.CancelFunc, error) {
	return func() {}, nil
}

// withCookedMode is a no-op on JS/WASM since there is no raw-mode TTY to restore.
func withCookedMode(fn func() error) error {
	return fn()
}
