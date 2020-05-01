package towercontroller

import (
	"testing"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

func TestEndProcess_Action(t *testing.T) {
	exp := 1
	as := (&EndProcess{Logger: logrus.New()}).Actions()

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
	if n := (&EndProcess{Logger: logrus.New()}).Next(); n != nil {
		t.Errorf("expected next state nil, got %s", statemachine.NameOf(n))
	}
}
