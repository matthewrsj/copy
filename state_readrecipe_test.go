package towercontroller

import (
	"testing"

	"bou.ke/monkey"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestRecipe_Action(t *testing.T) {
	exp := 1
	as := (&ReadRecipe{childLogger: zap.NewExample().Sugar()}).Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	lr := monkey.Patch(LoadRecipe, func(string, string, string) (traycontrollers.StepConfiguration, error) {
		return traycontrollers.StepConfiguration{}, nil
	})
	defer lr.Unpatch()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestRecipe_ActionNoRecipe(t *testing.T) {
	exp := 1
	as := (&ReadRecipe{childLogger: zap.NewExample().Sugar()}).Actions()

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
	if n := statemachine.NameOf((&ReadRecipe{childLogger: zap.NewExample().Sugar()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
