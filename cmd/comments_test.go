package cmd

import (
	"fmt"
	"strings"
	"testing"
)

// TestMaskSource checks the comment mask across language families: comment content
// survives at its exact offsets, code and strings blank to spaces, and the mask never
// changes the text's length or line breaks.
func TestMaskSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Path     string
		In       string
		WantKeep []string
		WantDrop []string
	}{{ // Test 0: Go line comment kept, code and string dropped.
		Path:     "a.go",
		In:       "// a robust plan\nvar s = \"seamless code\"\n",
		WantKeep: []string{"a robust plan"},
		WantDrop: []string{"var", "seamless"},
	}, { // Test 1: A comment marker inside a string is code.
		Path:     "a.go",
		In:       "s := \"// not a comment\" // a real comment\n",
		WantKeep: []string{"a real comment"},
		WantDrop: []string{"not a comment"},
	}, { // Test 2: Block comment content spans lines.
		Path:     "a.c",
		In:       "/* first line\nsecond line */ int x;\n",
		WantKeep: []string{"first line", "second line"},
		WantDrop: []string{"int x", "/*", "*/"},
	}, { // Test 3: Python hash comment kept, triple-quoted docstring dropped.
		Path:     "a.py",
		In:       "\"\"\"a # docstring\"\"\"\nx = 1  # trailing note\n",
		WantKeep: []string{"trailing note"},
		WantDrop: []string{"docstring", "x = 1"},
	}, { // Test 4: A Rust lifetime apostrophe does not swallow the comment.
		Path:     "a.rs",
		In:       "fn f<'a>(x: &'a str) {} // lifetime note\n",
		WantKeep: []string{"lifetime note"},
		WantDrop: []string{"str"},
	}, { // Test 5: SQL double-dash comment kept, quoted marker dropped.
		Path:     "a.sql",
		In:       "SELECT '-- not this' AS x -- but this\n",
		WantKeep: []string{"but this"},
		WantDrop: []string{"not this", "SELECT"},
	}, { // Test 6: A Makefile scans with hash comments.
		Path:     "Makefile",
		In:       "all: # build everything\n\tgo build .\n",
		WantKeep: []string{"build everything"},
		WantDrop: []string{"go build"},
	}, { // Test 7: An unterminated string gives up at the line break.
		Path:     "a.go",
		In:       "s := \"broken\n// still a comment\n",
		WantKeep: []string{"still a comment"},
		WantDrop: []string{"broken"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, ok := maskSource(test.Path, test.In)
			if !ok {
				t.Fatalf("maskSource(%q) not scannable, want ok", test.Path)
			}
			if len(got) != len(test.In) {
				t.Fatalf("mask changed length: %d != %d", len(got), len(test.In))
			}
			if strings.Count(got, "\n") != strings.Count(test.In, "\n") {
				t.Fatalf("mask changed line count")
			}
			for _, want := range test.WantKeep {
				at := strings.Index(test.In, want)
				if at < 0 || !strings.Contains(got, want) {
					t.Errorf("mask lost %q (%q)", want, got)
					continue
				}
				if got[at:at+len(want)] != want {
					t.Errorf("mask moved %q off its original offset (%q)", want, got)
				}
			}
			for _, drop := range test.WantDrop {
				if strings.Contains(got, drop) {
					t.Errorf("mask kept %q, want it blanked (%q)", drop, got)
				}
			}
		})
	}

	// A type with no comments is not scannable.
	if _, ok := maskSource("data.json", "{}"); ok {
		t.Error("maskSource on .json = ok, want not scannable")
	}
}
