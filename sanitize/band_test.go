package sanitize

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestBandOf checks the band boundaries, including both edges and out-of-range values.
func TestBandOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantBand Band
		In       int
	}{{ // Test 0: Zero is clean.
		In: 0, WantBand: BandLow,
	}, { // Test 1: Just under the mid boundary.
		In: 24, WantBand: BandLow,
	}, { // Test 2: The mid boundary itself.
		In: 25, WantBand: BandMid,
	}, { // Test 3: Just under the high boundary.
		In: 54, WantBand: BandMid,
	}, { // Test 4: The high boundary itself.
		In: 55, WantBand: BandHigh,
	}, { // Test 5: Saturated.
		In: 100, WantBand: BandHigh,
	}, { // Test 6: Above the scale still reads high rather than empty.
		In: 1000, WantBand: BandHigh,
	}, { // Test 7: Below the scale still reads low rather than empty.
		In: -1, WantBand: BandLow,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.WantBand, BandOf(test.In)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestScoreBand checks that the method reads the score's own value.
func TestScoreBand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantBand Band
		In       Score
	}{{ // Test 0: A clean score.
		In: Score{Value: 2}, WantBand: BandLow,
	}, { // Test 1: A mixed score.
		In: Score{Value: 40}, WantBand: BandMid,
	}, { // Test 2: A dense score.
		In: Score{Value: 85}, WantBand: BandHigh,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.WantBand, test.In.Band()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
