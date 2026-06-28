package hush

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/pkg/errors"
)

const jqUsage = `jq - Go implementation of jq (gojq builtin)

Synopsis:
  echo '{"foo": 128}' | jq '.foo'

Usage:
  jq [OPTIONS] [FILTER] [FILE...]

Options:
  -r, --raw-output              output raw strings
  -j, --join-output             implies -r with no newline delimiter
  -c, --compact-output          output without pretty-printing
  -s, --slurp                   read all inputs into an array
  -n, --null-input              use null as input value
  -e, --exit-status             exit 1 when the last value is false or null
  -R, --raw-input               read input as raw strings
      --arg name value          set a string value to a variable
      --argjson name value      set a JSON value to a variable
  -h, --help                    display this help
`

func jq(term *Console, args ...string) error {
	return handleJQ(context.Background(), args, term.Stdin, term.Stdout, term.Stderr)
}

func handleJQ(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var (
		rawOutput  bool
		compact    bool
		slurp      bool
		nullInput  bool
		exitStatus bool
		rawInput   bool
		joinOutput bool
		argNames   []string
		argValues  []any
	)

	// Parse flags and extract the query.
	var queryStr string
	var fileArgs []string
	i := 0 // start at 0 since args does not include command name
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Fprint(stdout, jqUsage)
			return nil
		case arg == "-r" || arg == "--raw-output":
			rawOutput = true
		case arg == "-j" || arg == "--join-output":
			joinOutput = true
			rawOutput = true
		case arg == "-c" || arg == "--compact-output":
			compact = true
		case arg == "-s" || arg == "--slurp":
			slurp = true
		case arg == "-n" || arg == "--null-input":
			nullInput = true
		case arg == "-e" || arg == "--exit-status":
			exitStatus = true
		case arg == "-R" || arg == "--raw-input":
			rawInput = true
		case arg == "--arg":
			if i+2 >= len(args) {
				fmt.Fprintf(stderr, "jq: --arg requires name and value\n")
				return errors.New("exit status 2")
			}
			argNames = append(argNames, "$"+args[i+1])
			argValues = append(argValues, args[i+2])
			i += 2
		case arg == "--argjson":
			if i+2 >= len(args) {
				fmt.Fprintf(stderr, "jq: --argjson requires name and value\n")
				return errors.New("exit status 2")
			}
			var val any
			if err := json.Unmarshal([]byte(args[i+2]), &val); err != nil {
				fmt.Fprintf(stderr, "jq: invalid JSON for --argjson %s: %s\n", args[i+1], err)
				return errors.New("exit status 2")
			}
			argNames = append(argNames, "$"+args[i+1])
			argValues = append(argValues, val)
			i += 2
		case arg == "--":
			i++
			for i < len(args) {
				fileArgs = append(fileArgs, args[i])
				i++
			}
			continue
		case strings.HasPrefix(arg, "-") && queryStr != "":
			fmt.Fprintf(stderr, "jq: unknown option: %s\n", arg)
			return errors.New("exit status 2")
		default:
			if queryStr == "" {
				queryStr = arg
			} else {
				fileArgs = append(fileArgs, arg)
			}
		}
		i++
	}

	if queryStr == "" {
		queryStr = "."
	}

	query, err := gojq.Parse(queryStr)
	if err != nil {
		fmt.Fprintf(stderr, "jq: %s\n", err)
		return errors.New("exit status 3")
	}

	opts := []gojq.CompilerOption{
		gojq.WithEnvironLoader(os.Environ),
	}
	if len(argNames) > 0 {
		opts = append(opts, gojq.WithVariables(argNames))
	}

	code, err := gojq.Compile(query, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "jq: %s\n", err)
		return errors.New("exit status 3")
	}

	inputs, err := readInputs(ctx, stdin, fileArgs, nullInput, rawInput, slurp)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		fmt.Fprintf(stderr, "jq: %s\n", err)
		return errors.New("exit status 2")
	}

	var lastFalsy bool
	for _, input := range inputs {
		iter := code.Run(input, argValues...)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				fmt.Fprintf(stderr, "jq: %s\n", err)
				return errors.New("exit status 5")
			}
			if exitStatus {
				lastFalsy = v == nil || v == false
			}
			if err := writeValue(stdout, v, rawOutput, compact, joinOutput); err != nil {
				return err
			}
		}
	}

	if exitStatus && lastFalsy {
		return errors.New("exit status 1")
	}
	return nil
}

func readInputs(ctx context.Context, stdin io.Reader, files []string, nullInput, rawInput, slurp bool) ([]any, error) {
	if nullInput {
		return []any{nil}, nil
	}

	var readers []io.Reader
	if len(files) > 0 {
		for _, f := range files {
			file, err := os.Open(f)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			readers = append(readers, file)
		}
	} else {
		readers = []io.Reader{stdin}
	}

	var vals []any
	for _, r := range readers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := io.ReadAll(ctxReader{ctx: ctx, r: r})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, err
		}

		if rawInput {
			lines := strings.Split(string(data), "\n")
			if slurp {
				vals = append(vals, strings.Join(lines, "\n"))
			} else {
				for _, line := range lines {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					if line != "" || !slurp {
						vals = append(vals, line)
					}
				}
			}
			continue
		}

		dec := json.NewDecoder(strings.NewReader(string(data)))
		var streamVals []any
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var v any
			if err := dec.Decode(&v); err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("parse error: %w", err)
			}
			streamVals = append(streamVals, v)
		}

		if slurp {
			vals = append(vals, streamVals)
		} else {
			vals = append(vals, streamVals...)
		}
	}

	if len(vals) == 0 {
		return []any{nil}, nil
	}
	return vals, nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr ctxReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

func writeValue(w io.Writer, v any, raw, compact, join bool) error {
	if raw {
		if s, ok := v.(string); ok {
			if _, err := fmt.Fprint(w, s); err != nil {
				return err
			}
			if !join {
				_, err := fmt.Fprint(w, "\n")
				return err
			}
			return nil
		}
	}

	var bs []byte
	var err error
	if compact {
		bs, err = gojq.Marshal(v)
	} else {
		bs, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		return err
	}
	if _, writeErr := w.Write(bs); writeErr != nil {
		return writeErr
	}
	_, err = fmt.Fprint(w, "\n")
	return err
}
