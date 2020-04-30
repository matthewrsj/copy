package towercontroller

import (
	"time"

	"stash.teslamotors.com/ctet/statemachine"
)

const _scanDeadline = 10 * time.Second

type FixtureBarcode struct {
	statemachine.Common

	tbc          trayBarcode
	fxbc         fixtureBarcode
	scanDeadline time.Time
	scanErr      error
}

func (f *FixtureBarcode) action() {
	var input string

	if input, f.scanErr = promptDeadline("scan fixture barcode", time.Now().Add(_scanDeadline)); f.scanErr != nil {
		f.SetLast(true)
		return
	}

	if f.fxbc, f.scanErr = newFixtureBarcode(input); f.scanErr != nil {
		f.SetLast(true)
		return
	}
}

func (f *FixtureBarcode) Actions() []func() {
	return []func(){
		f.action,
	}
}

func (f *FixtureBarcode) Next() statemachine.State {
	return &ProcessStep{
		fxbc: f.fxbc,
		tbc:  f.tbc,
	}
}
