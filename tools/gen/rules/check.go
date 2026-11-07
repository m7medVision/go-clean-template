package rules

import (
	"context"
	"errors"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
	"github.com/m7medvision/go-clean-template/tools/gen/plan"
)

// Check runs R2 to R5 on p, in that order, and returns the first rule that fails. R1 ran before:
// on the name before the plan was made, since planning needs a valid name, then on the identifiers
// the plan declares (IdentifiersFree). planErrs are what planning ran into: a file to edit that
// does not exist (R2) and places not found exactly once (R4). Any other error among them is
// returned before any rule runs.
func Check(ctx context.Context, r Repo, p *plan.Plan, planErrs []error) error {
	for _, err := range planErrs {
		if !errors.Is(err, ErrTargets) && !errors.Is(err, edit.ErrAnchor) {
			return err
		}
	}

	for _, err := range planErrs {
		if errors.Is(err, ErrTargets) {
			return err
		}
	}

	for _, check := range []func() error{
		func() error { return creatable(r, p.Created()) },
		func() error { return clean(ctx, r, p) },
		func() error { return anchors(planErrs) },
		func() error { return valid(p) },
	} {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}
