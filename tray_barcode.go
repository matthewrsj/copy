package towercontroller

import (
	"fmt"
	"regexp"
)

// TrayBarcode contains the components of the tray barcode
type TrayBarcode struct {
	SN string
	O  Orientation

	raw string
}

const _trayRegex = `^[0-9]{7,}[A-Da-d]$`

func isValidTrayBarcode(input string) error {
	r := regexp.MustCompile(_trayRegex)
	if !r.MatchString(input) {
		return fmt.Errorf("invalid tray barcode %s does not follow pattern \"%s\"", input, _trayRegex)
	}

	return nil
}

// NewTrayBarcode creates a new TrayBarcode from the input string.
func NewTrayBarcode(input string) (TrayBarcode, error) {
	if err := isValidTrayBarcode(input); err != nil {
		return TrayBarcode{}, fmt.Errorf("validate tray barcode: %v", err)
	}

	// the barcode is now valid, we know the last character is the orientation
	// and the first N characters are the serial number, we can access these directly without worrying about a panic.

	var (
		tbc TrayBarcode
		err error
	)

	if tbc.O, err = newOrientation(input[len(input)-1]); err != nil {
		return tbc, fmt.Errorf("parse orientation: %v", err)
	}

	tbc.SN = input[:len(input)-1]
	tbc.raw = input

	return tbc, nil
}
