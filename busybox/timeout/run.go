package timeout

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

func Run(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: timeout DURATION COMMAND [ARG]...")
	}

	dur, err := time.ParseDuration(args[1])
	if err != nil {
		return fmt.Errorf("timeout: invalid duration: %v", err)
	}

	cmd := exec.Command(args[2], args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("timeout: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-timer.C:
		cmd.Process.Kill()
		<-done // reap zombie
		return fmt.Errorf("timeout: command timed out")
	}
}
