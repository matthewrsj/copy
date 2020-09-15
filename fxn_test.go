package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestIDFromFXR(t *testing.T) {
	testCases := []struct {
		in  traycontrollers.FixtureBarcode
		out string
	}{
		{
			in: traycontrollers.FixtureBarcode{
				Tower: "hi",
				Fxn:   "there",
			},
			out: "hi-there",
		},
		{
			in: traycontrollers.FixtureBarcode{
				Tower: "",
				Fxn:   "there",
			},
			out: "-there",
		},
		{
			in: traycontrollers.FixtureBarcode{
				Tower: "",
				Fxn:   "",
			},
			out: "-",
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tc.out, IDFromFXR(tc.in))
		})
	}
}

func TestIDFromFXRString(t *testing.T) {
	testCases := []struct {
		in, out     string
		errExpected bool
	}{
		{in: "CM2-63010-01-01", out: "01-01"},
		{in: "CM2-63010-02-01", out: "02-01"},
		{in: "CM2-63010-02", errExpected: true},
		{in: "CM2-63010--01", errExpected: true},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			act, err := IDFromFXRString(tc.in)
			assert.Equal(t, tc.errExpected, err != nil)
			assert.Equal(t, tc.out, act)
		})
	}
}
