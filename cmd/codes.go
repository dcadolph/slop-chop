package cmd

// Exit codes for the slop-chop binary.
const (
	// codeFindings means check mode found slop.
	codeFindings = 1
	// codeError means a usage or IO problem stopped the run.
	codeError = 2
)
