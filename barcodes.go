package towercontroller

import (
	"errors"
	"fmt"
)

// Barcodes contains the Fixture and Tray barcodes
// that initiate a towercontroller state machine.
type Barcodes struct {
	Fixture         FixtureBarcode
	Tray            TrayBarcode
	ProcessStepName string
}

// ScanBarcodes prompts to scan the barcodes for tray and fixture and
// packages them into a Barcodes object.
func ScanBarcodes(c Configuration) (Barcodes, error) {
	var (
		bcs Barcodes
		err error
	)

	p, err := prompt("scan tray barcode", isValidTrayBarcode)
	if err != nil {
		return bcs, err
	}

	bcs.Tray, err = newTrayBarcode(p)
	if err != nil {
		return bcs, fmt.Errorf("parse tray barcode: %v", err)
	}

	input, err := prompt("scan fixture barcode", isValidFixtureBarcode)
	if err != nil {
		return bcs, err
	}

	bcs.Fixture, err = newFixtureBarcode(input)
	if err != nil {
		return bcs, fmt.Errorf("scan fixture barcode: %v", err)
	}

	bcs.ProcessStepName, err = getNextProcessStep(c.CellAPI, bcs.Tray.SN)
	if err != nil {
		return bcs, fmt.Errorf("get next process step: %v", err)
	}

	if !promptConfirm(fmt.Sprintf("next process step %s", bcs.ProcessStepName)) {
		return bcs, errors.New("process step canceled")
	}

	return bcs, nil
}
