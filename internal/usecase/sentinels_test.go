package usecase_test

import "errors"

// errInternalServErr is a repository failure, shared by the use-case tests in this package.
var errInternalServErr = errors.New("internal server error")
