package rewrite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestRewrite checks that Rewrite passes the text to the completer and trims the reply.
func TestRewrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		Reply      string
		WantResult string
		Want       error
	}{{ // Test 0: Reply is returned trimmed.
		In: "dirty", Reply: "  clean text\n", WantResult: "clean text", Want: nil,
	}, { // Test 1: Completer error is wrapped.
		In: "dirty", Reply: "", WantResult: "", Want: errBoom,
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var gotUser string
			c := CompleterFunc(func(_ context.Context, _, user string) (string, error) {
				gotUser = user
				if test.Want != nil {
					return "", test.Want
				}
				return test.Reply, nil
			})
			got, err := New(c).Rewrite(context.Background(), test.In)
			if !errors.Is(err, test.Want) {
				t.Fatalf("err = %v, want %v", err, test.Want)
			}
			if test.Want != nil {
				return
			}
			if gotUser != test.In {
				t.Errorf("user prompt = %q, want %q", gotUser, test.In)
			}
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBuildSystemTone checks that tone notes land in the system prompt.
func TestBuildSystemTone(t *testing.T) {
	t.Parallel()
	got := buildSystem([]string{"dry and direct"})
	if !strings.Contains(got, "dry and direct") {
		t.Errorf("system prompt missing tone note:\n%s", got)
	}
	if !strings.Contains(got, "em-dash") {
		t.Errorf("system prompt missing core instruction:\n%s", got)
	}
}

// TestNewNilPanics checks that New panics on a nil completer.
func TestNewNilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("New(nil): want panic")
		}
	}()
	New(nil)
}

// errBoom is a sentinel completer error for tests.
var errBoom = errors.New("boom")
