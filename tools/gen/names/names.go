// Package names derives every form of a domain or use-case name from its kebab-case singular:
// the Go type, package, file, table, REST path and stream prefix all come from one string.
package names

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"regexp"
	"strings"
)

// ErrInvalid is a name or plural that breaks rule R1.
var ErrInvalid = errors.New("R1 name")

// The pattern a name and its plural must match: kebab-case, starting with a letter.
var pattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// reserved reports whether ident names a package that generated files import or are generated
// into. A domain named after one would shadow it.
func reserved(ident string) bool {
	switch ident {
	case "aggregate", "apidoc", "apierr", "behavior", "command", "cqrs", "entity", "event", "generated",
		"gqlerr", "middleware", "model", "postgres", "queries", "query", "repo", "repomock", "rpcerr", "v1", "v2":
		return true
	default:
		return false
	}
}

// Forms are the ways one name is written.
type Forms struct {
	Kebab string // line-item: REST paths, operation IDs
	Go    string // LineItem: exported identifiers, proto messages, GraphQL types
	Lower string // lineItem: unexported identifiers, GraphQL fields
	Pkg   string // lineitem: Go packages and folders, stream prefixes
	Snake string // line_item: file names, tables
	Words string // line item: comments and messages
}

// Name is a singular name's forms, and its plural's.
type Name struct {
	Forms

	Plural Forms
}

// Parse derives every form of kebab and of its plural. The plural follows plain English rules
// unless plural is set. Either failing R1 is ErrInvalid.
func Parse(kebab, plural string) (Name, error) {
	if plural == "" {
		plural = pluralize(kebab)
	}

	singular, err := forms(kebab)
	if err != nil {
		return Name{}, err
	}

	many, err := forms(plural)
	if err != nil {
		return Name{}, fmt.Errorf("plural: %w", err)
	}

	if many.Pkg == singular.Pkg {
		return Name{}, fmt.Errorf("%w: the plural %q must differ from %q; pass plural=", ErrInvalid, plural, kebab)
	}

	return Name{Forms: singular, Plural: many}, nil
}

func forms(kebab string) (Forms, error) {
	if !pattern.MatchString(kebab) {
		return Forms{}, fmt.Errorf("%w: %q is not kebab-case starting with a letter (%s)", ErrInvalid, kebab, pattern)
	}

	words := strings.Split(kebab, "-")
	f := Forms{
		Kebab: kebab,
		Pkg:   strings.Join(words, ""),
		Snake: strings.Join(words, "_"),
		Words: strings.Join(words, " "),
	}

	for i, w := range words {
		f.Go += strings.ToUpper(w[:1]) + w[1:]

		if i == 0 {
			f.Lower = w
		} else {
			f.Lower += strings.ToUpper(w[:1]) + w[1:]
		}
	}

	for _, ident := range []string{f.Pkg, f.Lower} {
		switch {
		case token.IsKeyword(ident):
			return Forms{}, fmt.Errorf("%w: %q gives %q, a Go keyword", ErrInvalid, kebab, ident)
		case types.Universe.Lookup(ident) != nil:
			return Forms{}, fmt.Errorf("%w: %q gives %q, a predeclared Go name", ErrInvalid, kebab, ident)
		case reserved(ident):
			return Forms{}, fmt.Errorf("%w: %q gives %q, a package generated files import", ErrInvalid, kebab, ident)
		}
	}

	return f, nil
}

// pluralize applies plain English rules to the last word: -s, -x, -z, -ch and -sh take -es, a
// consonant before -y becomes -ies, and everything else takes -s.
func pluralize(kebab string) string {
	switch {
	case strings.HasSuffix(kebab, "s"), strings.HasSuffix(kebab, "x"), strings.HasSuffix(kebab, "z"),
		strings.HasSuffix(kebab, "ch"), strings.HasSuffix(kebab, "sh"):
		return kebab + "es"
	case strings.HasSuffix(kebab, "y") && len(kebab) > 1 && !strings.ContainsRune("aeiou", rune(kebab[len(kebab)-2])):
		return kebab[:len(kebab)-1] + "ies"
	default:
		return kebab + "s"
	}
}
