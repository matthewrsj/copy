package towercontroller

import (
	"errors"
	"fmt"

	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// Barcodes contains the Fixture and Tray barcodes
// that initiate a towercontroller state machine.
type Barcodes struct {
	Fixture         traycontrollers.FixtureBarcode
	Tray            traycontrollers.TrayBarcode
	ProcessStepName string
	InProgress      bool
}

// ScanBarcodes prompts to scan the barcodes for tray and fixture and
// packages them into a Barcodes object.
func ScanBarcodes(caClient *cellapi.Client) (Barcodes, error) {
	var (
		bcs Barcodes
		err error
	)

	p, err := prompt("scan tray barcode", traycontrollers.IsValidTrayBarcode)
	if err != nil {
		return bcs, err
	}

	bcs.Tray, err = traycontrollers.NewTrayBarcode(p)
	if err != nil {
		return bcs, fmt.Errorf("parse tray barcode: %v", err)
	}

	input, err := prompt("scan fixture barcode", traycontrollers.IsValidFixtureBarcode)
	if err != nil {
		return bcs, err
	}

	bcs.Fixture, err = traycontrollers.NewFixtureBarcode(input)
	if err != nil {
		return bcs, fmt.Errorf("scan fixture barcode: %v", err)
	}

	bcs.ProcessStepName, err = caClient.GetNextProcessStep(bcs.Tray.SN)
	if err != nil {
		return bcs, fmt.Errorf("get next process step: %v", err)
	}

	if !promptConfirm(fmt.Sprintf("next process step %s", bcs.ProcessStepName)) {
		return bcs, errors.New("process step canceled")
	}

	return bcs, nil
}
