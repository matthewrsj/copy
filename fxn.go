package towercontroller

import (
	"fmt"

	"stash.teslamotors.com/rr/traycontrollers"
)

// IDFromFXR combines column and level numbers for fixture IDs
func IDFromFXR(fxr traycontrollers.FixtureBarcode) string {
	return fmt.Sprintf("%s-%s", fxr.Tower, fxr.Fxn)
}
