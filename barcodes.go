package towercontroller

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
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
	ManualMode      bool
	MockCellAPI     bool

	TransactID    string
	RecipeName    string
	RecipeVersion int
	StepConf      traycontrollers.StepConfiguration
}

const _mockedFormRequest = "FORM_CYCLE"

// ScanBarcodes prompts to scan the barcodes for tray and fixture and
// packages them into a Barcodes object.
func ScanBarcodes(caClient *cellapi.Client, mockCellAPI bool, logger *zap.SugaredLogger) (Barcodes, error) {
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

	if !mockCellAPI {
		bcs.ProcessStepName, err = caClient.GetNextProcessStep(bcs.Tray.SN)
		if err != nil {
			return bcs, fmt.Errorf("get next process step: %v", err)
		}
	} else {
		logger.Warnf("cell API mocked, skipping GetNextProcessStep and using %s", _mockedFormRequest)
		bcs.ProcessStepName = _mockedFormRequest
	}

	if !promptConfirm(fmt.Sprintf("next process step %s", bcs.ProcessStepName)) {
		return bcs, errors.New("process step canceled")
	}

	bcs.MockCellAPI = mockCellAPI

	return bcs, nil
}
