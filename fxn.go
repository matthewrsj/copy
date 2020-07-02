package towercontroller

import (
	"fmt"

	"stash.teslamotors.com/rr/traycontrollers"
)

// IDFromFXR combines column and level numbers for fixture IDs
func IDFromFXR(fxr traycontrollers.FixtureBarcode) string {
	return fmt.Sprintf("%s-%s", fxr.Tower, fxr.Fxn)
}

// IDFromFXRString gets the configuration ID from a complete FXR string
func IDFromFXRString(fxr string) (string, error) {
	fxbc, err := traycontrollers.NewFixtureBarcode(fxr)
	if err != nil {
		return "", err
	}

	return IDFromFXR(fxbc), nil
}
