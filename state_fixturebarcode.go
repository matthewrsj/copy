package towercontroller

import (
	"time"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

const _scanDeadline = 10 * time.Second

type FixtureBarcode struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc          trayBarcode
	fxbc         fixtureBarcode
	scanDeadline time.Time
	scanErr      error
}

func (f *FixtureBarcode) action() {
	var input string

	f.Logger.Info("waiting for fixture barcode scan")

	if input, f.scanErr = promptDeadline("scan fixture barcode", time.Now().Add(_scanDeadline)); f.scanErr != nil {
		f.Logger.Error(f.scanErr)
		f.SetLast(true)

		return
	}

	if f.fxbc, f.scanErr = newFixtureBarcode(input); f.scanErr != nil {
		f.Logger.Error(f.scanErr)
		f.SetLast(true)

		return
	}

	f.Logger.WithFields(logrus.Fields{
		"location":    f.fxbc.location,
		"aisle":       f.fxbc.aisle,
		"tower":       f.fxbc.tower,
		"fixture_num": f.fxbc.fxn,
		"raw":         f.fxbc.raw,
	}).Info("fixture barcode scanned")
}

func (f *FixtureBarcode) Actions() []func() {
	return []func(){
		f.action,
	}
}

func (f *FixtureBarcode) Next() statemachine.State {
	next := &ProcessStep{
		Logger: f.Logger,
		fxbc:   f.fxbc,
		tbc:    f.tbc,
	}
	f.Logger.WithField("tray", f.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}
