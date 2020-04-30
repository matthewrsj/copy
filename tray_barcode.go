package towercontroller

import (
	"fmt"
	"regexp"
)

type trayBarcode struct {
	sn string
	o  orientation

	raw string
}

const _trayRegex = `^[0-9]{7,}[A-Da-d]$`

func isValidTrayBarcode(input string) bool {
	r := regexp.MustCompile(_trayRegex)
	return r.MatchString(input)
}

func newTrayBarcode(input string) (trayBarcode, error) {
	if !isValidTrayBarcode(input) {
		return trayBarcode{}, fmt.Errorf("invalid tray barcode %s does not follow pattern \"%s\"", input, _trayRegex)
	}

	// the barcode is now valid, we know the last character is the orientation
	// and the first N characters are the serial number, we can access these directly without worrying about a panic.

	var (
		tbc trayBarcode
		err error
	)

	if tbc.o, err = newOrientation(input[len(input)-1]); err != nil {
		return tbc, fmt.Errorf("parse orientation: %v", err)
	}

	tbc.sn = input[:len(input)-1]
	tbc.raw = input

	return tbc, nil
}
