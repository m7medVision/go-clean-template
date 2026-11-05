package main

import (
	"context"
	"fmt"
	"io"

	"github.com/m7medvision/go-clean-template/tools/gen/plan"
	"github.com/m7medvision/go-clean-template/tools/gen/rules"
)

// regenerated are the paths the regenerators rewrite after a write: make proto-v1, graphql, sqlc,
// mock, openapi, web-api and web-graphql. Rule R3 requires them clean too, and the undo restores
// them.
func regenerated() []string {
	return []string{
		"docs/openapi/v1.json",
		"docs/proto/v1",
		"internal/controller/graphql/generated",
		"internal/controller/graphql/model",
		"internal/repo/persistent/queries",
		"internal/repo/repomock",
		"web/src/api/schema.gen.ts",
		"web/src/graphql",
	}
}

// execute checks the plan pl made against R2 to R5 and, unless it is a dry run, writes it (R6) and
// prints the undo. A dry run prints the plan, then fails if a rule would. Once the plan is written,
// or a dry run passes, it prints the planner's checklist: what you write next.
func execute(ctx context.Context, r rules.Repo, root plan.FS, pl *planner, dryRun bool, stdout io.Writer) error {
	p := pl.plan
	checked := rules.Check(ctx, r, p, regenerated(), pl.errs)

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

	written, err := plan.Write(root, changes)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "%s: created %d, edited %d.\nTo undo it, and what the regenerators do next:\n  %s\n\n%s",
		p.Title(), len(p.Created()), len(written.Edited), plan.Undo(written.Edited, regenerated(), written.Removable), pl.next)

	return err
}
