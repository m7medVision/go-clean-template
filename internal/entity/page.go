package entity

const (
	// DefaultPageLimit is the size of a page whose caller asks for none (a Limit of 0).
	DefaultPageLimit = 10
	// MaxPageLimit is the largest page a list serves.
	MaxPageLimit = 100
)

// Page asks for Limit items of a list after its first Offset. A Limit of 0 is DefaultPageLimit.
type Page struct {
	Limit  int
	Offset int
}

// Validate refuses a negative offset, and a limit below 0 or above MaxPageLimit.
func (p Page) Validate() error {
	if p.Limit < 0 || p.Limit > MaxPageLimit || p.Offset < 0 {
		return ErrInvalidPage
	}

	return nil
}

// Size is how many items the page holds at most.
func (p Page) Size() int {
	if p.Limit == 0 {
		return DefaultPageLimit
	}

	return p.Limit
}

// List is one page of a list and how many items the list holds in all.
type List[T any] struct {
	Items []T
	Total int
}
