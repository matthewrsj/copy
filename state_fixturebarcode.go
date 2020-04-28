package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type FixtureBarcode struct {
	statemachine.Common
}

func (f *FixtureBarcode) action() {}

func (f *FixtureBarcode) Actions() []func() {
	return []func(){
		f.action,
	}
}

func (f *FixtureBarcode) Next() statemachine.State {
	return &ProcessStep{}
}
