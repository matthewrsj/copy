package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type TrayBarcode struct {
	statemachine.Common
}

func (t *TrayBarcode) action() {}

func (t *TrayBarcode) Actions() []func() {
	return []func(){
		t.action,
	}
}

func (t *TrayBarcode) Next() statemachine.State {
	return &FixtureBarcode{}
}
