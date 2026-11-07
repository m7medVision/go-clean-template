// Command openapi writes the REST OpenAPI document to stdout. It reads no environment: the document
// depends only on the code and the version it states, which -version sets (make openapi passes
// APP_VERSION).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/m7medvision/go-clean-template/internal/app"
)

var errNoVersion = errors.New("-version is required")

func main() {
	version := flag.String("version", "", "the API version the document states (required)")

	flag.Parse()

	if err := run(*version); err != nil {
		fmt.Fprintln(os.Stderr, "openapi:", err)
		os.Exit(1)
	}
}

func run(version string) error {
	if version == "" {
		return errNoVersion
	}

	doc, err := app.OpenAPI(version)
	if err != nil {
		return fmt.Errorf("app.OpenAPI: %w", err)
	}

	if _, err = os.Stdout.Write(append(doc, '\n')); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
