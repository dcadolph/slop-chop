package rewrite

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestJudge checks verdict parsing, fenced-JSON tolerance, and the error paths.
func TestJudge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Reply       string
		CompErr     error
		WantVerdict Verdict
		WantErrSub  string
	}{{ // Test 0: A faithful verdict parses.
		Reply:       `{"faithful": true, "issues": []}`,
		WantVerdict: Verdict{Faithful: true},
	}, { // Test 1: An issue parses with all fields.
		Reply: `{"faithful": false, "issues":[` +
			`{"kind":"changed","was":"99.9%","now":"99%","note":"figure changed"}]}`,
		WantVerdict: Verdict{
			Faithful: false,
			Issues:   []Issue{{Kind: "changed", Was: "99.9%", Now: "99%", Note: "figure changed"}},
		},
	}, { // Test 2: JSON wrapped in a code fence is tolerated.
		Reply:       "```json\n{\"faithful\": true, \"issues\": []}\n```",
		WantVerdict: Verdict{Faithful: true},
	}, { // Test 3: A reply with no JSON object is an error.
		Reply: "looks fine to me", WantErrSub: "no JSON object",
	}, { // Test 4: A malformed object is a decode error.
		Reply: `{"faithful": maybe}`, WantErrSub: "decode verdict",
	}, { // Test 5: A completer error is wrapped.
		CompErr: errBoom, WantErrSub: "judge",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			fake := CompleterFunc(func(_ context.Context, _, _ string) (string, error) {
				return test.Reply, test.CompErr
			})
			got, err := NewJudge(fake).Judge(t.Context(), "orig", "rewrite")
			if test.WantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), test.WantErrSub) {
					t.Fatalf("err = %v, want substring %q", err, test.WantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("Judge: %v", err)
			}
			if diff := cmp.Diff(test.WantVerdict, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestJudgeSendsBothTexts checks that the original and the rewrite both reach the model.
func TestJudgeSendsBothTexts(t *testing.T) {
	t.Parallel()
	var gotUser string
	fake := CompleterFunc(func(_ context.Context, _, user string) (string, error) {
		gotUser = user
		return `{"faithful": true, "issues": []}`, nil
	})
	if _, err := NewJudge(fake).Judge(t.Context(), "the original", "the rewrite"); err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if !strings.Contains(gotUser, "the original") || !strings.Contains(gotUser, "the rewrite") {
		t.Errorf("user prompt = %q, want both texts", gotUser)
	}
}

// TestNewJudgeNilPanics checks the nil-completer guard.
func TestNewJudgeNilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("NewJudge(nil) did not panic")
		}
	}()
	NewJudge(nil)
}
