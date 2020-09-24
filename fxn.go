package towercontroller

import (
	"fmt"

	"stash.teslamotors.com/rr/cdcontroller"
)

// IDFromFXR combines column and level numbers for fixture IDs
func IDFromFXR(fxr cdcontroller.FixtureBarcode) string {
	return fmt.Sprintf("%s-%s", fxr.Tower, fxr.Fxn)
}

// IDFromFXRString gets the configuration ID from a complete FXR string
func IDFromFXRString(fxr string) (string, error) {
	fxbc, err := cdcontroller.NewFixtureBarcode(fxr)
	if err != nil {
		return "", err
	}

	return IDFromFXR(fxbc), nil
}
