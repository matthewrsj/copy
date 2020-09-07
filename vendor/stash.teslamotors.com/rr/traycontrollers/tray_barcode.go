package traycontrollers

import (
	"fmt"
	"regexp"
)

// TrayBarcode contains the components of the tray barcode
type TrayBarcode struct {
	SN  string
	O   Orientation
	Raw string
}

const _trayRegex = `^[0-9A-Za-z \-]{7,}[A-Da-d]$`

// IsValidTrayBarcode returns an error if the input string is not a valid
// barcode for a tray.
func IsValidTrayBarcode(input string) error {
	r := regexp.MustCompile(_trayRegex)
	if !r.MatchString(input) {
		return fmt.Errorf("invalid tray barcode %s does not follow pattern \"%s\"", input, _trayRegex)
	}

	return nil
}

// NewTrayBarcode creates a new TrayBarcode from the input string.
func NewTrayBarcode(input string) (TrayBarcode, error) {
	if err := IsValidTrayBarcode(input); err != nil {
		return TrayBarcode{}, fmt.Errorf("validate tray barcode: %v", err)
	}

	// the barcode is now valid, we know the last character is the orientation
	// and the first N characters are the serial number, we can access these directly without worrying about a panic.

	var (
		tbc TrayBarcode
		err error
	)

	if tbc.O, err = NewOrientation(input[len(input)-1]); err != nil {
		return tbc, fmt.Errorf("parse orientation: %v", err)
	}

	tbc.SN = input[:len(input)-1]
	tbc.Raw = input

	return tbc, nil
}
