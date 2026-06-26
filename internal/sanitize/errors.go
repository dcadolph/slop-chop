package sanitize

import "errors"

// ErrCompile means a profile could not be turned into rules.
var ErrCompile = errors.New("profile compile")
