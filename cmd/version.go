package cmd

import "runtime/debug"

// version is stamped at build time via ldflags:
// -ldflags "-X github.com/dcadolph/slop-chop/cmd.version=v1.2.3".
//
//nolint:gochecknoglobals // Build-time stamp.
var version = "dev"

// resolveVersion returns the stamped build version. When the binary is
// installed with `go install` rather than built through the Makefile the stamp
// is absent, so it falls back to the module version from the build info.
func resolveVersion() string {
	if version != "dev" && version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
