package cmd

import "testing"

// TestResolveVersion checks that a build-time stamp wins over the build-info
// fallback, and that an unstamped build reports dev when no module version is
// available.
func TestResolveVersion(t *testing.T) {
	tests := []struct {
		Stamp string
		Want  string
	}{{ // Test 0: a stamped version is returned as-is.
		Stamp: "v1.2.3", Want: "v1.2.3",
	}, { // Test 1: an unstamped test binary has no module version, so dev.
		Stamp: "dev", Want: "dev",
	}}
	for testNum, test := range tests {
		// Not parallel: resolveVersion reads the package-level version var.
		orig := version
		version = test.Stamp
		got := resolveVersion()
		version = orig
		if got != test.Want {
			t.Errorf("test %d: want %q got %q", testNum, test.Want, got)
		}
	}
}
