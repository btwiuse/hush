package hush

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/btwiuse/u-root/pkg/core"
	"github.com/btwiuse/u-root/pkg/core/base64"
	"github.com/btwiuse/u-root/pkg/core/chmod"
	"github.com/btwiuse/u-root/pkg/core/cp"
	"github.com/btwiuse/u-root/pkg/core/find"
	"github.com/btwiuse/u-root/pkg/core/gzip"
	"github.com/btwiuse/u-root/pkg/core/ls"
	"github.com/btwiuse/u-root/pkg/core/mkdir"
	"github.com/btwiuse/u-root/pkg/core/mktemp"
	"github.com/btwiuse/u-root/pkg/core/mv"
	"github.com/btwiuse/u-root/pkg/core/rm"
	"github.com/btwiuse/u-root/pkg/core/shasum"
	"github.com/btwiuse/u-root/pkg/core/tar"
	"github.com/btwiuse/u-root/pkg/core/touch"
	"github.com/btwiuse/u-root/pkg/core/xargs"
	"github.com/pkg/errors"
	lnpkg "github.com/btwiuse/hush/busybox/ln"
)

type builtinFunc func(term console, args ...string) error

var (
	builtins = map[string]builtinFunc{}
)

var commandBuilders = map[string]func() core.Command{
	"chmod":  func() core.Command { return chmod.New() },
	"cp":     func() core.Command { return cp.New() },
	"find":   func() core.Command { return find.New() },
	"ls":     func() core.Command { return ls.New() },
	"mkdir":  func() core.Command { return mkdir.New() },
	"mv":     func() core.Command { return mv.New() },
	"rm":     func() core.Command { return rm.New() },
	"touch":  func() core.Command { return touch.New() },
	"xargs":  func() core.Command { return xargs.New() },
	"base64": func() core.Command { return base64.New() },
	"gzcat":  func() core.Command { return gzip.New("gzcat") },
	"gzip":   func() core.Command { return gzip.New("gzip") },
	"gunzip": func() core.Command { return gzip.New("gunzip") },
	"mktemp": func() core.Command { return mktemp.New() },
	"shasum": func() core.Command { return shasum.New() },
	"tar":    func() core.Command { return tar.New() },
}

func coreUtilBuiltin(name string) builtinFunc {
	return func(term console, args ...string) error {
		newCmd, ok := commandBuilders[name]
		if !ok {
			return fmt.Errorf("%s: unknown command", name)
		}

		cmd := newCmd()
		cmd.SetIO(getconsoleStdin(term), term.Stdout(), term.Stderr())
		wd, _ := os.Getwd()
		cmd.SetWorkingDir(wd)
		cmd.SetLookupEnv(func(key string) (string, bool) {
			return os.LookupEnv(key)
		})
		return cmd.RunContext(context.Background(), args...)
	}
}

func init() {
	for k, v := range map[string]builtinFunc{
		"cat":    cat,
		"cat2":    cat,
		"chmod":  coreUtilBuiltin("chmod"),
		"clear":  clear,
		"cp":     coreUtilBuiltin("cp"),
		"curl":   curl,
		"env":    env,
		"find":   coreUtilBuiltin("find"),
		"ls":     coreUtilBuiltin("ls"),
		"ln":     ln,
		"mkdir":  coreUtilBuiltin("mkdir"),
		"mv":     coreUtilBuiltin("mv"),
		"rm":     coreUtilBuiltin("rm"),
		"rmdir":  rmdir,
		"touch":  coreUtilBuiltin("touch"),
		"which":  which,
		"xargs":  coreUtilBuiltin("xargs"),
		"base64": coreUtilBuiltin("base64"),
		"gzip":   coreUtilBuiltin("gzip"),
		"gunzip": coreUtilBuiltin("gunzip"),
		"mktemp": coreUtilBuiltin("mktemp"),
		"shasum": coreUtilBuiltin("shasum"),
		"tar":    coreUtilBuiltin("tar"),
	} {
		builtins[k] = v
	}
}

func rmdir(term console, args ...string) error {
	if len(args) == 0 {
		return errors.New("Not enough args")
	}
	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return &os.PathError{Path: path, Op: "remove", Err: syscall.ENOTDIR}
		}
		err = os.Remove(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func which(term console, args ...string) error {
	if len(args) == 0 {
		return errors.New("Not enough args")
	}
	for _, arg := range args {
		path, err := exec.LookPath(arg)
		if err != nil {
			return err
		}
		fmt.Fprintln(term.Stdout(), path)
	}
	return nil
}

func curl(term console, args ...string) error {
	set := flag.NewFlagSet("curl", flag.ContinueOnError)
	head := set.Bool("I", false, "Fetch headers only")
	follow := set.Bool("L", false, "Follow redirects")
	output := set.Bool("O", false, "Save to remote-named file")
	if err := set.Parse(args); err != nil {
		return err
	}
	urls := set.Args()
	if len(urls) == 0 {
		return errors.New("No URL provided")
	}

	client := &http.Client{}
	if *follow {
		client.CheckRedirect = nil
	} else {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	for _, rawURL := range urls {
		if !strings.Contains(rawURL, "://") {
			rawURL = "https://" + rawURL
		}

		method := http.MethodGet
		if *head {
			method = http.MethodHead
		}

		req, err := http.NewRequest(method, rawURL, nil)
		if err != nil {
			return errors.Wrap(err, rawURL)
		}

		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrap(err, rawURL)
		}

		fmt.Fprintf(term.Stdout(), "HTTP/%d.%d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.Status)
		resp.Header.Write(term.Stdout())
		fmt.Fprintln(term.Stdout())

		if *head {
			resp.Body.Close()
			continue
		}

		if *output {
			u, err := url.Parse(rawURL)
			if err != nil {
				resp.Body.Close()
				return errors.Wrap(err, rawURL)
			}
			filename := filepath.Base(u.Path)
			if filename == "/" || filename == "." || filename == "" {
				filename = "index.html"
			}
			f, err := os.Create(filename)
			if err != nil {
				resp.Body.Close()
				return err
			}
			_, err = io.Copy(f, resp.Body)
			resp.Body.Close()
			f.Close()
			if err != nil {
				return err
			}
		} else {
			_, err = io.Copy(term.Stdout(), resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func clear(term console, args ...string) error {
	clearWriter(term.Stdout())
	return nil
}

func env(term console, args ...string) error {
	var kv []string
	const equals = '='
	for i, arg := range args {
		if !strings.ContainsRune(arg, equals) {
			args = args[i:]
			break
		}
		kv = append(kv, arg)
	}

	if len(args) == 0 {
		for _, e := range os.Environ() {
			fmt.Fprintln(term.Stdout(), e)
		}
		return nil
	}

	cmd := exec.Command(args[0], args[1:]...) // nolint:gosec
	cmd.Stdout = term.Stdout()
	cmd.Stderr = term.Stderr()
	cmd.Env = append(os.Environ(), kv...)
	return cmd.Run()
}

func ln(term console, args ...string) error {
	return lnpkg.Run(args)
}
