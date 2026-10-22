package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Source says where the template to generate from comes from.
type Source struct {
	// Ref is an explicit tag, branch or commit to fetch, even inside a checkout.
	Ref string
	// Dir is the working directory; a template at or above it is used as is.
	Dir string
	// Version is the CLI's own build version (debug.ReadBuildInfo), fetched outside a checkout.
	Version string
	// Module is the template's module path, which names its GitHub repository.
	Module string
	// Fetch downloads the archive at url into dir and returns the template's root.
	Fetch func(ctx context.Context, url, dir string) (string, error)
}

// Opened is a template ready to generate from.
type Opened struct {
	Root string
	// Local is a template checkout, as opposed to a fetched copy.
	Local bool
	// Ref is the ref a fetched copy came from.
	Ref string
	// OutBase is where projects go by default: empty for a local template (its scaffold/out),
	// the checkout's scaffold/out for a ref fetched inside one, the working directory otherwise.
	OutBase string
	tmp     string
}

// Close removes a fetched copy. A local template is left alone.
func (o Opened) Close() error {
	if o.tmp == "" {
		return nil
	}

	if err := os.RemoveAll(o.tmp); err != nil {
		return fmt.Errorf("scaffold - Opened.Close - os.RemoveAll: %w", err)
	}

	return nil
}

// Open finds the template. An explicit Ref is fetched; otherwise a template at or above Dir is
// used; otherwise the ref of the CLI's own version is fetched, so a CLI run as
// `go run …/cmd/create@v0.4.0` generates from v0.4.0.
func (s Source) Open(ctx context.Context) (Opened, error) {
	checkout, err := FindRoot(s.Dir)
	inCheckout := err == nil

	switch {
	case s.Ref != "":
		return s.fetch(ctx, s.Ref, checkout)
	case inCheckout:
		return Opened{Root: checkout, Local: true}, nil
	}

	if ref := RefFromVersion(s.Version); ref != "" {
		return s.fetch(ctx, ref, "")
	}

	return Opened{}, fmt.Errorf("%w; run it as go run %s/cmd/create@latest, or pass --ref", ErrNoTemplate, s.Module)
}

// fetch downloads ref. checkout is the template checkout around Dir, if any: projects then still
// go in its scaffold/out.
func (s Source) fetch(ctx context.Context, ref, checkout string) (Opened, error) {
	outBase := filepath.Join(checkout, filepath.FromSlash(_outDir))
	if checkout == "" {
		var err error
		if outBase, err = filepath.Abs(s.Dir); err != nil {
			return Opened{}, fmt.Errorf("scaffold - Source.Open - filepath.Abs: %w", err)
		}
	}

	tmp, err := os.MkdirTemp("", "scaffold-template-")
	if err != nil {
		return Opened{}, fmt.Errorf("scaffold - Source.Open - os.MkdirTemp: %w", err)
	}

	root, err := s.Fetch(ctx, ArchiveURL(s.Module, ref), tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)

		return Opened{}, err
	}

	return Opened{Root: root, Ref: ref, OutBase: outBase, tmp: tmp}, nil
}

// RefFromVersion is the template ref a CLI of this build version was built from: a release or
// pre-release tag as is, the commit of a pseudo-version, and nothing for a local build
// ("(devel)", or a "+dirty" working tree that matches no commit).
func RefFromVersion(version string) string {
	if !semver.IsValid(version) || strings.Contains(version, "+") {
		return ""
	}

	if module.IsPseudoVersion(version) {
		rev, err := module.PseudoVersionRev(version)
		if err != nil {
			return ""
		}

		return rev
	}

	return version
}
