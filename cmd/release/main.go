// Command release prints the template's next version, or its release notes, from the Conventional
// Commits since the last release tag. Run it through `make release-version` and
// `make release-notes`; see docs/releasing.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/m7medvision/go-clean-template/internal/release"
)

func main() {
	notes := flag.Bool("notes", false, "print the release notes instead of the version")

	flag.Parse()

	if err := run(context.Background(), *notes); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

// run prints the next version, or nothing (and why, on stderr) when HEAD needs no release.
func run(ctx context.Context, notes bool) error {
	state, err := release.Read(ctx, ".")
	if err != nil {
		return err
	}

	if notes {
		return write(release.Notes(state.Commits))
	}

	version, reason, err := release.Decide(state)
	if err != nil {
		return err
	}

	if version == "" {
		fmt.Fprintln(os.Stderr, "release: nothing to release:", reason)

		return nil
	}

	return write(version + "\n")
}

func write(s string) error {
	if _, err := os.Stdout.WriteString(s); err != nil {
		return fmt.Errorf("release - os.Stdout.WriteString: %w", err)
	}

	return nil
}
