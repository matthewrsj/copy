package towercontroller

import (
	"testing"

	"github.com/sirupsen/logrus"

	"stash.teslamotors.com/ctet/statemachine/v2"
)

func TestRecipe_Action(t *testing.T) {
	exp := 1
	as := (&ReadRecipe{Logger: logrus.New()}).Actions()

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

func TestRecipe_Next(t *testing.T) {
	exp := "StartProcess"
	if n := statemachine.NameOf((&ReadRecipe{Logger: logrus.New()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
