package main

import (
	"context"
	"fmt"
	"io"

	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// execute checks the plan pl made against R2 to R5 and, unless it is a dry run, writes it (R6) and
// prints the undo, made from the tree as it was before the write. A dry run prints the plan, then
// fails if a rule would. Once the plan is written, or a dry run passes, it prints the planner's
// checklist: what you write next.
func execute(ctx context.Context, r rules.Repo, root plan.FS, pl *planner, dryRun bool, stdout io.Writer) error {
	p := pl.plan
	checked := rules.Check(ctx, r, p, pl.errs)

	if dryRun {
		if err := p.DryRun(stdout); err != nil {
			return err
		}

		if checked != nil {
			return checked
		}

		_, err := fmt.Fprintf(stdout, "\nEvery rule passes. Run it again without dry-run=1 to write it.\n\n%s", pl.next)

		return err
	}

	if checked != nil {
		return checked
	}

	changes, err := p.Changes()
	if err != nil {
		return err
	}

	undo, err := rules.Undo(ctx, r)
	if err != nil {
		return err
	}

	if err := plan.Write(root, changes); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "%s: created %d, edited %d.\nTo undo it, and what the regenerators do next:\n  %s\n\n%s",
		p.Title(), len(p.Created()), len(p.Edited()), undo, pl.next)

	return err
}
