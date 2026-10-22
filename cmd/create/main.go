// Command create generates a project from this template with only the features picked. Run it
// through `make scaffold`; see docs/scaffold.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/m7medvision/go-clean-template/internal/scaffold"
)

// _maxTemplate caps a fetched template's extracted size; the template is a few megabytes.
const _maxTemplate = 512 << 20

// _fetchTimeout bounds downloading the template.
const _fetchTimeout = 5 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
}

func run() error {
	cmd, ref, matrix := parseFlags()
	ctx := context.Background()
	runner := scaffold.ShellRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}

	if matrix {
		root, err := scaffold.FindRoot(".")
		if err != nil {
			return err
		}

		return scaffold.Matrix(ctx, root, os.Stdout, runner)
	}

	src, err := openSource(ctx, ref)
	if err != nil {
		return err
	}
	defer src.Close()

	cmd.Source = src.Root

	if !src.Local {
		// Outside a checkout, the project goes in the working directory.
		if cmd.OutBase, err = os.Getwd(); err != nil {
			return fmt.Errorf("create - os.Getwd: %w", err)
		}

		fmt.Fprintln(os.Stderr, "create: using the template at", src.Ref)
	}

	if interactive(&cmd) {
		ok, err := ask(ctx, src.Root, &cmd)
		if err != nil {
			return err
		}

		if !ok {
			fmt.Fprintln(os.Stderr, "create: nothing generated")

			return nil
		}
	}

	return cmd.Run(ctx, os.Stdout, runner)
}

func parseFlags() (cmd scaffold.Command, ref string, matrix bool) {
	var with, without string

	flag.StringVar(&cmd.Preset, "preset", "", "start from this preset (api, api-grpc, fullstack, everything)")
	flag.StringVar(&with, "with", "", "comma-separated features to add")
	flag.StringVar(&without, "without", "", "comma-separated features to leave out")
	flag.StringVar(&cmd.Name, "name", "", "project name, and its directory")
	flag.StringVar(&cmd.Module, "module", "", "Go module path of the generated project")
	flag.StringVar(&cmd.Out, "out", "", "output directory instead of ./<name> (scaffold/out/<name> in the template)")
	flag.StringVar(&ref, "ref", "", "generate from this tag, branch or commit of the template (default: this CLI's version)")
	flag.BoolVar(&cmd.Force, "force", false, "replace the output directory if it is not empty")
	flag.BoolVar(&cmd.NoRegen, "no-regen", false, "skip regenerating code (sqlc, mocks, gqlgen, ...)")
	flag.BoolVar(&cmd.Check, "check", false, "run make pre-commit in the generated project")
	flag.StringVar(&cmd.Then, "then", "", "a command to run in the generated project afterwards (make dev ...)")
	flag.BoolVar(&matrix, "matrix", false, "generate and check every preset, then print a summary")
	flag.Parse()

	cmd.With, cmd.Without = list(with), list(without)

	return cmd, ref, matrix
}

// openSource finds the template: --ref, else the checkout around the working directory, else
// the template at this CLI's own version.
func openSource(ctx context.Context, ref string) (scaffold.Opened, error) {
	src := scaffold.Source{Ref: ref, Dir: "."}

	if info, ok := debug.ReadBuildInfo(); ok {
		src.Version, src.Module = info.Main.Version, info.Main.Path
	}

	fetcher := scaffold.Fetcher{Client: &http.Client{Timeout: _fetchTimeout}, MaxBytes: _maxTemplate}
	src.Fetch = fetcher.Fetch

	return src.Open(ctx)
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
