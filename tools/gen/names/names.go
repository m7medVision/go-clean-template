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

// A letter right after a digit, inside a word: protoc-gen-go capitalizes it.
var letterAfterDigit = regexp.MustCompile(`\d[a-z]`)

// _choosePlural is what to do about a plural that breaks R1.
const _choosePlural = "choose another plural with plural="

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
	if err == nil {
		err = protocAlike(kebab, &singular, "put a dash after the digit, as in "+
			letterAfterDigit.ReplaceAllStringFunc(kebab, func(s string) string { return s[:1] + "-" + s[1:] }))
	}

	if err != nil {
		return Name{}, err
	}

	many, err := forms(plural)
	if err == nil {
		err = protocAlike(plural, &many, _choosePlural)
	}

	if err == nil {
		err = listFieldAlike(plural, &many)
	}

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
		f.Go += upperFirst(w)

		if i == 0 {
			f.Lower = w
		} else {
			f.Lower += upperFirst(w)
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

	if err := initialismFree(kebab, words); err != nil {
		return Forms{}, err
	}

	return f, nil
}

// initialismFree checks that no word of kebab is one gqlgen or revive writes in capitals in a Go
// name, as both write api in APIKey, where the generator writes ApiKey. The tools split a word
// where its letters end, so a word is looked up by its letters: http2 is HTTP2 to them, and utf8
// is Utf8, since utf is no initialism.
func initialismFree(kebab string, words []string) error {
	for _, w := range words {
		letters := strings.TrimRight(w, "0123456789")

		if who, as := initialism(letters); who != "" {
			return fmt.Errorf("%w: %q has the word %q, which %s as %s in Go names, and the generator as %s, so what it "+
				"generates would not build or lint: use another word, or join it to the word beside it without a dash",
				ErrInvalid, kebab, w, who, as+w[len(letters):], upperFirst(w))
		}
	}

	return nil
}

// initialism says who writes word in capitals in a Go name, and how, for a word gqlgen or revive
// knows as an initialism; who is empty for any other. The lists are those of the versions the
// app's go.mod pins: gqlgen v0.17.98 (templates.CommonInitialisms), which names the methods and
// models of the GraphQL resolvers, and revive v1.17.0 (var-naming), which lints every Go name.
// Their UTF8 is kept as they have it, though no word's letters match it.
func initialism(word string) (who, as string) {
	switch upper := strings.ToUpper(word); upper {
	case "ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON",
		"LHS", "QPS", "RAM", "RHS", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "UID",
		"URI", "URL", "UTF8", "UUID", "VM", "XML", "XMPP", "XSRF", "XSS":
		return "gqlgen and revive write", upper
	case "AWS", "CSV", "GCP", "ICMP", "KVK", "PDF", "PGP", "QR", "SVG":
		return "gqlgen writes", upper
	case "IDS":
		return "revive writes", "IDs"
	default:
		return "", ""
	}
}

// protocAlike checks that protoc-gen-go writes the Go names it derives from kebab, a name or a
// plural with the forms f, as the generator does: it capitalizes a letter right after a digit, as
// in Mp3PlayerResponse, where the generator writes Mp3playerResponse. fix says what to do.
func protocAlike(kebab string, f *Forms, fix string) error {
	at := letterAfterDigit.FindString(kebab)
	if at == "" {
		return nil
	}

	return fmt.Errorf("%w: %q has a letter right after a digit (%s), which protoc-gen-go capitalizes in Go names (%s) "+
		"and the generator does not (%s), so the gRPC code would not build: %s",
		ErrInvalid, kebab, at, letterAfterDigit.ReplaceAllStringFunc(f.Go, strings.ToUpper), f.Go, fix)
}

// listFieldAlike checks that protoc-gen-go writes the Go name of the list field, which the plural
// names, as the generator does: it keeps the underscore before a word that starts with a digit,
// as in Items_2 for items-2, where the generator writes Items2.
func listFieldAlike(plural string, f *Forms) error {
	var (
		field    strings.Builder
		digitLed string
	)

	for i, w := range strings.Split(plural, "-") {
		if i > 0 && strings.ContainsRune("0123456789", rune(w[0])) {
			field.WriteString("_")

			if digitLed == "" {
				digitLed = w
			}
		}

		field.WriteString(upperFirst(w))
	}

	if digitLed == "" {
		return nil
	}

	return fmt.Errorf("%w: %q has the word %q, which starts with a digit: protoc-gen-go keeps the underscore before it "+
		"in the Go name of the list field (%s) and the generator does not (%s), so the gRPC code would not build: %s",
		ErrInvalid, plural, digitLed, field.String(), f.Go, _choosePlural)
}

// upperFirst is w with its first letter in capitals.
func upperFirst(w string) string {
	return strings.ToUpper(w[:1]) + w[1:]
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
