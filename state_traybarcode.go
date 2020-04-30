package towercontroller

import (
	"time"

	"stash.teslamotors.com/ctet/statemachine"
)

type TrayBarcode struct {
	statemachine.Common

	tbc          trayBarcode
	scanErr      error
	scanDeadline time.Time
}

func (t *TrayBarcode) action() {
	if t.tbc, t.scanErr = newTrayBarcode(prompt("scan tray barcode")); t.scanErr != nil {
		t.SetLast(true)
	}
}

func (t *TrayBarcode) Actions() []func() {
	return []func(){
		t.action,
	}
}

func (t *TrayBarcode) Next() statemachine.State {
	return &FixtureBarcode{
		tbc:          t.tbc,
		scanDeadline: t.scanDeadline,
	}
}
