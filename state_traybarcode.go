package towercontroller

import (
	"time"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type TrayBarcode struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc          trayBarcode
	scanErr      error
	scanDeadline time.Time
}

func (t *TrayBarcode) action() {
	t.Logger.Info("waiting for tray barcode scan")

	if t.tbc, t.scanErr = newTrayBarcode(prompt("scan tray barcode")); t.scanErr != nil {
		t.Logger.Error(t.scanErr)
		t.SetLast(true)
	}

	t.Logger.WithFields(logrus.Fields{
		"SN":          t.tbc.sn,
		"orientation": t.tbc.o,
		"raw":         t.tbc.raw,
	}).Info("tray barcode scanned")
}

func (t *TrayBarcode) Actions() []func() {
	return []func(){
		t.action,
	}
}

func (t *TrayBarcode) Next() statemachine.State {
	next := &FixtureBarcode{
		Logger:       t.Logger,
		tbc:          t.tbc,
		scanDeadline: t.scanDeadline,
	}
	t.Logger.WithField("tray", t.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}
