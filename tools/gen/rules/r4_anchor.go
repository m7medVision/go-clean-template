package rules

import (
	"errors"

	"github.com/m7medvision/go-clean-template/tools/gen/edit"
)

// Anchors checks R4: no edit failed to find its place exactly once. It returns every such failure
// among planErrs, joined, so one run names them all.
func Anchors(planErrs []error) error {
	var failed []error

	for _, err := range planErrs {
		if errors.Is(err, edit.ErrAnchor) {
			failed = append(failed, err)
		}
	}

	return errors.Join(failed...)
}
