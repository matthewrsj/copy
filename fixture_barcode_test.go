package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// nolint:scopelint // over-reports on test table usage
func Test_newFixtureBarcode(t *testing.T) {
	testCases := []struct {
		in          string
		out         FixtureBarcode
		errExpected bool
	}{
		{
			in: "A-B-C-D",
			out: FixtureBarcode{
				Location: "A",
				Aisle:    "B",
				Tower:    "C",
				Fxn:      "D",
				raw:      "A-B-C-D",
			},
			errExpected: false,
		},
		{
			in: "a-b-c-d",
			out: FixtureBarcode{
				Location: "a",
				Aisle:    "b",
				Tower:    "c",
				Fxn:      "d",
				raw:      "a-b-c-d",
			},
			errExpected: false,
		},
		{
			in: "1-2-3-4",
			out: FixtureBarcode{
				Location: "1",
				Aisle:    "2",
				Tower:    "3",
				Fxn:      "4",
				raw:      "1-2-3-4",
			},
			errExpected: false,
		},
		{
			in: "SWIFT-01-A-04",
			out: FixtureBarcode{
				Location: "SWIFT",
				Aisle:    "01",
				Tower:    "A",
				Fxn:      "04",
				raw:      "SWIFT-01-A-04",
			},
			errExpected: false,
		},
		{
			in:          "A-B-C",
			errExpected: true,
		},
		{
			in:          "A-B-C-D-E",
			errExpected: true,
		},
		{
			in:          "ABCD",
			errExpected: true,
		},
		{
			in:          "",
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			actual, err := NewFixtureBarcode(tc.in)
			if err != nil != tc.errExpected {
				t.Fatalf("got error: %v; expected error: %v", err != nil, tc.errExpected)
			}

			if err != nil {
				return
			}

			if actual.Location != tc.out.Location ||
				actual.Aisle != tc.out.Aisle ||
				actual.Fxn != tc.out.Fxn ||
				actual.Tower != tc.out.Tower ||
				actual.raw != tc.out.raw {
				t.Errorf("got %#v; expect %#v", actual, tc.out)
			}
		})
	}
}

func Test_isValidFixtureBarcode(t *testing.T) {
	testCases := []struct {
		in    string
		valid bool
	}{
		{"SWIFT-01-A-02", true},
		{"01-01-02-02", true},
		{"01-02-02", false},
		{"02-02", false},
		{"02", false},
		{"", false},
		{"---", false},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.valid, isValidFixtureBarcode(tc.in) == nil)
		})
	}
}
