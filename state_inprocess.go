package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type InProcess struct {
	statemachine.Common
}

func (i *InProcess) action() {}

func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

func (i *InProcess) Next() statemachine.State {
	return &EndProcess{}
}
