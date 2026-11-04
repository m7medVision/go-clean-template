package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/m7medvision/go-clean-template/tools/gen/names"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// newDomain plans the files of a whole domain and writes them, or prints them on a dry run.
func newDomain(ctx context.Context, req domainRequest, stdout io.Writer) error {
	n, err := names.Parse(req.name, req.plural)
	if err != nil {
		return err
	}

	r := rules.Open(req.root)
	if err := rules.DomainFree(r, &n); err != nil {
		return err
	}

	p := plan.New("new-domain " + n.Kebab)

	root, err := os.OpenRoot(req.root)
	if err != nil {
		return fmt.Errorf("open %s: %w", req.root, err)
	}
	defer root.Close()

	return execute(ctx, r, root, p, nil, req.dryRun, stdout)
}
