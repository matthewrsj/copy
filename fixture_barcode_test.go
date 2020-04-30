package towercontroller

import "testing"

// length ok for unit tests, scopelint over-reports on test table usage
// nolint: funlen,scopelint
func Test_newFixtureBarcode(t *testing.T) {
	testCases := []struct {
		in          string
		out         fixtureBarcode
		errExpected bool
	}{
		{
			in: "A-B-C-D",
			out: fixtureBarcode{
				location: "A",
				aisle:    "B",
				tower:    "C",
				fxn:      "D",
				raw:      "A-B-C-D",
			},
			errExpected: false,
		},
		{
			in: "a-b-c-d",
			out: fixtureBarcode{
				location: "a",
				aisle:    "b",
				tower:    "c",
				fxn:      "d",
				raw:      "a-b-c-d",
			},
			errExpected: false,
		},
		{
			in: "1-2-3-4",
			out: fixtureBarcode{
				location: "1",
				aisle:    "2",
				tower:    "3",
				fxn:      "4",
				raw:      "1-2-3-4",
			},
			errExpected: false,
		},
		{
			in: "SWIFT-01-A-04",
			out: fixtureBarcode{
				location: "SWIFT",
				aisle:    "01",
				tower:    "A",
				fxn:      "04",
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
			actual, err := newFixtureBarcode(tc.in)
			if err != nil != tc.errExpected {
				t.Fatalf("got error: %v; expected error: %v", err != nil, tc.errExpected)
			}

			if err != nil {
				return
			}

			if actual.location != tc.out.location ||
				actual.aisle != tc.out.aisle ||
				actual.fxn != tc.out.fxn ||
				actual.tower != tc.out.tower ||
				actual.raw != tc.out.raw {
				t.Errorf("got %#v; expect %#v", actual, tc.out)
			}
		})
	}
}
