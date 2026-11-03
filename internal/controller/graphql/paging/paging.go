// Package paging checks the limit and offset arguments of GraphQL list fields.
package paging

const _minLimit = 1

// Valid reports whether limit is at least 1 and offset is not negative.
func Valid(limit, offset int) bool {
	return limit >= _minLimit && offset >= 0
}

// ValidUpTo is Valid with limit also at most maxLimit.
func ValidUpTo(limit, offset, maxLimit int) bool {
	return Valid(limit, offset) && limit <= maxLimit
}
