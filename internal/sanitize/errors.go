package sanitize

import "errors"

// ErrCompile means a profile could not be turned into rules.
var ErrCompile = errors.New("profile compile")

// ErrDialect means a profile named a spelling dialect the engine does not know.
var ErrDialect = errors.New("unknown dialect")
