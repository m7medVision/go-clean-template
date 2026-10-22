// Command create generates a project from this template with only the features picked. Run it
// through `make scaffold`; see docs/scaffold.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
)

func main() {
	var (
		cmd           scaffold.Command
		with, without string
		matrix        bool
	)

	flag.StringVar(&cmd.Preset, "preset", "", "start from this preset (api, api-grpc, fullstack, everything)")
	flag.StringVar(&with, "with", "", "comma-separated features to add")
	flag.StringVar(&without, "without", "", "comma-separated features to leave out")
	flag.StringVar(&cmd.Name, "name", "", "project name, and its directory under scaffold/out")
	flag.StringVar(&cmd.Module, "module", "", "Go module path of the generated project")
	flag.StringVar(&cmd.Out, "out", "", "output directory instead of scaffold/out/<name>")
	flag.BoolVar(&cmd.Force, "force", false, "replace the output directory if it is not empty")
	flag.BoolVar(&cmd.NoRegen, "no-regen", false, "skip regenerating code (sqlc, mocks, gqlgen, ...)")
	flag.BoolVar(&cmd.Check, "check", false, "run make pre-commit in the generated project")
	flag.StringVar(&cmd.Then, "then", "", "a command to run in the generated project afterwards (make dev ...)")
	flag.BoolVar(&matrix, "matrix", false, "generate and check every preset, then print a summary")
	flag.Parse()

	cmd.With, cmd.Without = list(with), list(without)

	ctx := context.Background()
	runner := scaffold.ShellRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}

	if matrix {
		root, err := scaffold.FindRoot(".")
		if err == nil {
			err = scaffold.Matrix(ctx, root, os.Stdout, runner)
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}

		return
	}

	if interactive(&cmd) {
		ok, err := ask(ctx, &cmd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}

		if !ok {
			fmt.Fprintln(os.Stderr, "create: nothing generated")

			return
		}
	}

	if err := cmd.Run(ctx, os.Stdout, runner); err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
}

// interactive reports whether to open the form: no selection flags, and a terminal on stdin.
func interactive(cmd *scaffold.Command) bool {
	if cmd.Preset != "" || len(cmd.With) > 0 || len(cmd.Without) > 0 || cmd.Name != "" || cmd.Module != "" || cmd.Out != "" {
		return false
	}

	info, err := os.Stdin.Stat()

	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// list splits a comma-separated flag value, ignoring blanks.
func list(value string) []string {
	var ids []string

	for id := range strings.SplitSeq(value, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}
