package jsonutil

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestMarshal checks compact and pretty encoding.
func TestMarshal(t *testing.T) {
	t.Parallel()
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	tests := []struct {
		In         payload
		WantResult string
		Pretty     bool
	}{{ // Test 0: Compact encoding has no whitespace.
		In: payload{Name: "a", Count: 1}, Pretty: false,
		WantResult: `{"name":"a","count":1}`,
	}, { // Test 1: Pretty encoding indents with two spaces.
		In: payload{Name: "a", Count: 1}, Pretty: true,
		WantResult: "{\n  \"name\": \"a\",\n  \"count\": 1\n}",
	}}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := Marshal(test.In, test.Pretty)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if diff := cmp.Diff(test.WantResult, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
