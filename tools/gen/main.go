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

	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

const (
	_exitOK     = 0
	_exitFailed = 1
	_exitUsage  = 2
)

const _usage = `usage:
  gen new-domain  --name <name> [--plural <plural>] [--transports rest,grpc,graphql] [--root <dir>] [--dry-run]
  gen new-usecase --domain <domain> --name <name> --kind command|query [--plural <domain plural>]
                  [--method <method>] [--path <path>] [--transports rest,grpc,graphql] [--root <dir>] [--dry-run]
`

var errUsage = errors.New("usage")

// domainRequest is what make new-domain asks for.
type domainRequest struct {
	root   string
	name   string
	plural string
	// transports are the transports the domain gets, in the order rules.Transports lists them.
	transports []string
	dryRun     bool
	// version names the domain's migration; empty for the time the command runs.
	version string
}

// usecaseRequest is what make new-usecase asks for.
type usecaseRequest struct {
	root string
	// domain is the domain's name, as make new-domain was given it, and plural its plural when
	// make new-domain was given that too.
	domain, plural string
	name           string
	kind           string
	// method and path are the REST operation's, or empty for the defaults.
	method, path string
	// transports are the transports the use case gets, in the order rules.Transports lists them;
	// nil for the domain's own.
	transports []string
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

		return newDomain(ctx, &req, stdout)
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
	var (
		req        domainRequest
		transports string
	)

	fs := newFlagSet("new-domain", stderr)
	fs.StringVar(&req.root, "root", ".", "the repository root")
	fs.StringVar(&req.name, "name", "", "the domain name, kebab-case and singular")
	fs.StringVar(&req.plural, "plural", "", "the plural, when English rules get it wrong")
	fs.StringVar(&transports, "transports", "", "comma-separated, default rest,grpc,graphql")
	fs.BoolVar(&req.dryRun, "dry-run", false, "print the plan and write nothing")

	if err := parseFlags(fs, args); err != nil {
		return req, err
	}

	if req.name == "" {
		fmt.Fprintln(stderr, "gen: new-domain needs --name")

		return req, errUsage
	}

	var err error

	req.transports, err = parseTransports(transports, stderr)

	return req, err
}

// parseTransports reads a --transports value: a comma-separated list drawn from rules.Transports,
// or every one of them when it is empty. It returns them in that order, once each; a transport it
// does not know is a usage error.
func parseTransports(value string, stderr io.Writer) ([]string, error) {
	known := rules.Transports()
	if value == "" {
		return known, nil
	}

	asked := strings.Split(value, ",")
	for i, t := range asked {
		asked[i] = strings.TrimSpace(t)

		if !slices.Contains(known, asked[i]) {
			fmt.Fprintf(stderr, "gen: unknown transport %q: --transports takes %s\n", asked[i], strings.Join(known, ","))

			return nil, errUsage
		}
	}

	return slices.DeleteFunc(known, func(t string) bool { return !slices.Contains(asked, t) }), nil
}

func parseUsecase(args []string, stderr io.Writer) (usecaseRequest, error) {
	var (
		req        usecaseRequest
		transports string
	)

	fs := newFlagSet("new-usecase", stderr)
	fs.StringVar(&req.root, "root", ".", "the repository root")
	fs.StringVar(&req.domain, "domain", "", "the existing domain, as make new-domain was given it")
	fs.StringVar(&req.plural, "plural", "", "the domain's plural, when make new-domain was given one")
	fs.StringVar(&req.name, "name", "", "the use case, kebab-case: archive-task")
	fs.StringVar(&req.kind, "kind", "", "command or query")
	fs.StringVar(&req.method, "method", "", "the REST method, default POST for a command and GET for a query")
	fs.StringVar(&req.path, "path", "", "the REST path, default derived from the name")
	fs.StringVar(&transports, "transports", "", "comma-separated, default the domain's own")
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

	if req.kind != _kindCommand && req.kind != _kindQuery {
		fmt.Fprintf(stderr, "gen: --kind is command or query, not %q\n", req.kind)

		return req, errUsage
	}

	if err := parseRoute(&req, stderr); err != nil {
		return req, err
	}

	if transports == "" {
		return req, nil
	}

	var err error

	req.transports, err = parseTransports(transports, stderr)

	return req, err
}

// parseRoute checks the REST method and path a use case was given, if any. The method is one of
// GET, POST, PUT, PATCH and DELETE, in any case; it is kept in capitals. The path starts with a
// slash and has no parameter but {id}, which names the record the use case acts on.
func parseRoute(req *usecaseRequest, stderr io.Writer) error {
	if req.method != "" {
		req.method = strings.ToUpper(req.method)

		if !slices.Contains([]string{"GET", "POST", "PUT", "PATCH", "DELETE"}, req.method) {
			fmt.Fprintf(stderr, "gen: --method is GET, POST, PUT, PATCH or DELETE, not %q\n", req.method)

			return errUsage
		}
	}

	if req.path == "" {
		return nil
	}

	rest := strings.ReplaceAll(req.path, "{id}", "")
	if !strings.HasPrefix(req.path, "/") || strings.ContainsAny(rest, "{} ") || strings.Count(req.path, "{id}") > 1 {
		fmt.Fprintf(stderr, "gen: --path starts with / and has no parameter but one {id}, not %q\n", req.path)

		return errUsage
	}

	return nil
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
