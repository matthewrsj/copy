package towercontroller

import "testing"

// length ok for unit tests, scopelint over-reports on test table usage
// nolint: funlen,scopelint
func Test_isValidTrayBarcode(t *testing.T) {
	testCases := []struct {
		in          string
		out         trayBarcode
		errExpected bool
	}{
		{
			in: "00000000A",
			out: trayBarcode{
				sn:  "00000000",
				o:   _orientA,
				raw: "00000000A",
			},
			errExpected: false,
		},
		{
			in: "0000000A",
			out: trayBarcode{
				sn:  "0000000",
				o:   _orientA,
				raw: "0000000A",
			},
			errExpected: false,
		},
		{
			in: "00000000000000000B",
			out: trayBarcode{
				sn:  "00000000000000000",
				o:   _orientB,
				raw: "00000000000000000B",
			},
			errExpected: false,
		},
		{
			in: "00000000a",
			out: trayBarcode{
				sn:  "00000000",
				o:   _orientA,
				raw: "00000000a",
			},
			errExpected: false,
		},
		{
			in:          "000A",
			errExpected: true,
		},
		{
			in:          "000",
			errExpected: true,
		},
		{
			in:          "00000000",
			errExpected: true,
		},
		{
			in:          "00000000E",
			errExpected: true,
		},
		{
			in:          "",
			errExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			actual, err := newTrayBarcode(tc.in)
			if err != nil != tc.errExpected {
				t.Fatalf("got error: %v; expected error: %v", err != nil, tc.errExpected)
			}

			if err != nil {
				return
			}

			if actual.sn != tc.out.sn ||
				actual.o != tc.out.o ||
				actual.raw != tc.out.raw {
				t.Errorf("got %#v; expect %#v", actual, tc.out)
			}
		})
	}
}