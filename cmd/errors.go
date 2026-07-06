package cmd

import "errors"

// errFindings signals that check mode found slop. Execute maps it to exit code 1 so the
// exit lifecycle stays in one place and the commands stay testable.
var errFindings = errors.New("findings")
