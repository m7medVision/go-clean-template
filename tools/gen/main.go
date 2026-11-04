// Command gen writes the files a new domain or use case needs: every layer, with its package,
// imports, types and signatures, and handler bodies that return entity.ErrNotImplemented. It
// checks every rule before it writes anything (docs/specs/generator.md).
//
// It is its own module, so no binary or image of the app includes it. Run it through
// make new-domain and make new-usecase, which also run the regenerators its output needs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

const (
	_exitOK     = 0
	_exitFailed = 1
	_exitUsage  = 2
)

const _usage = `usage:
  gen new-domain  --name <name> [--plural <plural>] [--transports rest,grpc,graphql] [--root <dir>] [--dry-run]
  gen new-usecase --domain <domain> --name <name> --kind command|query
                  [--method <method>] [--path <path>] [--transports rest,grpc,graphql] [--root <dir>] [--dry-run]
`

var errUsage = errors.New("usage")

// domainRequest is what make new-domain asks for.
type domainRequest struct {
	root       string
	name       string
	plural     string
	transports string
	dryRun     bool
}

// usecaseRequest is what make new-usecase asks for.
type usecaseRequest struct {
	root       string
	domain     string
	name       string
	kind       string
	method     string
	path       string
	transports string
	dryRun     bool
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one command line and returns the exit code: 0 when it succeeded, 1 when a rule
// failed or a write did, and 2 for a bad command line.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	err := dispatch(ctx, args, stdout, stderr)

	switch {
	case err == nil:
		return _exitOK
	case errors.Is(err, errUsage):
		fmt.Fprint(stderr, _usage)

		return _exitUsage
	default:
		fmt.Fprintf(stderr, "gen: %v\n", err)

		return _exitFailed
	}
}

func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}

	switch args[0] {
	case "new-domain":
		req, err := parseDomain(args[1:], stderr)
		if err != nil {
			return err
		}

		return newDomain(ctx, req, stdout)
	case "new-usecase":
		req, err := parseUsecase(args[1:], stderr)
		if err != nil {
			return err
		}

		return newUsecase(ctx, &req, stdout)
	default:
		fmt.Fprintf(stderr, "gen: unknown command %q\n", args[0])

		return errUsage
	}
}

func parseDomain(args []string, stderr io.Writer) (domainRequest, error) {
	var req domainRequest

	fs := newFlagSet("new-domain", stderr)
	fs.StringVar(&req.root, "root", ".", "the repository root")
	fs.StringVar(&req.name, "name", "", "the domain name, kebab-case and singular")
	fs.StringVar(&req.plural, "plural", "", "the plural, when English rules get it wrong")
	fs.StringVar(&req.transports, "transports", "", "comma-separated, default rest,grpc,graphql")
	fs.BoolVar(&req.dryRun, "dry-run", false, "print the plan and write nothing")

	if err := parseFlags(fs, args); err != nil {
		return req, err
	}

	if req.name == "" {
		fmt.Fprintln(stderr, "gen: new-domain needs --name")

		return req, errUsage
	}

	return req, nil
}

func parseUsecase(args []string, stderr io.Writer) (usecaseRequest, error) {
	var req usecaseRequest

	fs := newFlagSet("new-usecase", stderr)
	fs.StringVar(&req.root, "root", ".", "the repository root")
	fs.StringVar(&req.domain, "domain", "", "the existing domain")
	fs.StringVar(&req.name, "name", "", "the use case, kebab-case: archive-task")
	fs.StringVar(&req.kind, "kind", "", "command or query")
	fs.StringVar(&req.method, "method", "", "the REST method, default POST for a command and GET for a query")
	fs.StringVar(&req.path, "path", "", "the REST path, default derived from the name")
	fs.StringVar(&req.transports, "transports", "", "comma-separated, default the domain's own")
	fs.BoolVar(&req.dryRun, "dry-run", false, "print the plan and write nothing")

	if err := parseFlags(fs, args); err != nil {
		return req, err
	}

	var missing []string

	for flagName, value := range map[string]string{"domain": req.domain, "name": req.name, "kind": req.kind} {
		if value == "" {
			missing = append(missing, "--"+flagName)
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		fmt.Fprintf(stderr, "gen: new-usecase needs %s\n", strings.Join(missing, ", "))

		return req, errUsage
	}

	if req.kind != "command" && req.kind != "query" {
		fmt.Fprintf(stderr, "gen: --kind is command or query, not %q\n", req.kind)

		return req, errUsage
	}

	return req, nil
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}

	return fs
}

// parseFlags parses args, which must hold flags only.
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return errUsage
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(fs.Output(), "gen: unexpected argument %q\n", fs.Arg(0))

		return errUsage
	}

	return nil
}
