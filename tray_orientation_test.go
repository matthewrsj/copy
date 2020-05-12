package towercontroller

import (
	"testing"
)

// nolint:scopelint // over-reports on test table usage
func Test_newOrientation(t *testing.T) {
	// nolint:maligned // order of this struct is for test readability, not efficiency
	testCases := []struct {
		in          byte
		out         Orientation
		errExpected bool
	}{
		{
			in:          'A',
			out:         _orientA,
			errExpected: false,
		},
		{
			in:          'B',
			out:         _orientB,
			errExpected: false,
		},
		{
			in:          'C',
			out:         _orientC,
			errExpected: false,
		},
		{
			in:          'D',
			out:         _orientD,
			errExpected: false,
		},
		{
			in:          'a',
			out:         _orientA,
			errExpected: false,
		},
		{
			in:          'b',
			out:         _orientB,
			errExpected: false,
		},
		{
			in:          'c',
			out:         _orientC,
			errExpected: false,
		},
		{
			in:          'd',
			out:         _orientD,
			errExpected: false,
		},
		{
			in:          'w',
			errExpected: true,
		},
		{
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.in), func(t *testing.T) {
			actual, err := newOrientation(tc.in)
			if err != nil != tc.errExpected {
				t.Fatalf("got error: %v; expected error: %v", err != nil, tc.errExpected)
			}

			if actual != tc.out {
				t.Errorf("expected %v got %v", tc.out, actual)
			}
		})
	}
}
