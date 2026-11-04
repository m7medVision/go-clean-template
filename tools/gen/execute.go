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

// execute checks p against R2 to R5 and, unless it is a dry run, writes it (R6) and prints the
// undo. A dry run prints the plan, then fails if a rule would. planErrs are what planning ran
// into.
func execute(ctx context.Context, r rules.Repo, root plan.FS, p *plan.Plan, planErrs []error, dryRun bool, stdout io.Writer) error {
	checked := rules.Check(ctx, r, p, regenerated(), planErrs)

	if dryRun {
		if err := p.DryRun(stdout); err != nil {
			return err
		}

		if checked == nil {
			_, err := fmt.Fprintln(stdout, "\nEvery rule passes. Run it again without dry-run=1 to write it.")

			return err
		}

		return checked
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

	_, err = fmt.Fprintf(stdout, "%s: created %d, edited %d.\nTo undo it, and what the regenerators do next:\n  %s\n",
		p.Title(), len(p.Created()), len(written.Edited), plan.Undo(written.Edited, regenerated(), written.Removable))

	return err
}
