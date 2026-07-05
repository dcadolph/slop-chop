package main

import "errors"

// errFindings signals that check mode found slop. main maps it to exit code 1 so the
// exit lifecycle stays in main and run stays testable.
var errFindings = errors.New("findings")
