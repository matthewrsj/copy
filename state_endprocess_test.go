package towercontroller

import (
	"testing"

	"stash.teslamotors.com/ctet/statemachine"
)

func TestEndProcess_Action(t *testing.T) {
	exp := 1
	as := (&EndProcess{}).Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestEndProcess_Next(t *testing.T) {
	exp := "EndProcess"
	if n := statemachine.NameOf((&EndProcess{}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
