package main

import (
	"context"
	"fmt"
	"io"
)

// newUsecase plans one command or query on an existing domain and writes it, or prints it on a
// dry run.
func newUsecase(_ context.Context, req *usecaseRequest, stdout io.Writer) error {
	_, err := fmt.Fprintf(stdout, "new-usecase %s %s (dry run): nothing planned yet\n", req.domain, req.name)

	return err
}
