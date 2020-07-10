package towercontroller

import (
	"testing"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
)

func TestProcessStep_Action(t *testing.T) {
	psState := ProcessStep{
		Logger:        zap.NewExample().Sugar(),
		CellAPIClient: cellapi.NewClient("test"),
		fxrInfo:       &FixtureInfo{},
	}
	psState.SetContext(Barcodes{})
	as := psState.Actions()

	exp := 1
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

func TestProcessStep_ActionManual(t *testing.T) {
	psState := ProcessStep{
		Logger:        zap.NewExample().Sugar(),
		CellAPIClient: cellapi.NewClient("test"),
		fxrInfo:       &FixtureInfo{},
	}
	psState.SetContext(Barcodes{
		MockCellAPI: true,
	})

	as := psState.Actions()

	exp := 1
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

func TestProcessStep_ActionBadContext(t *testing.T) {
	psState := ProcessStep{
		Logger:  zap.NewExample().Sugar(),
		fxrInfo: &FixtureInfo{},
	}

	as := psState.Actions()

	exp := 1
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

func TestProcessStep_Next(t *testing.T) {
	exp := "StartProcess"
	if n := statemachine.NameOf((&ProcessStep{Logger: zap.NewExample().Sugar()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}

func TestProcessStep_NextIP(t *testing.T) {
	exp := "InProcess"
	if n := statemachine.NameOf((&ProcessStep{Logger: zap.NewExample().Sugar(), inProgress: true}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}

func TestProcessStep_NextManual(t *testing.T) {
	exp := "ReadRecipe"
	if n := statemachine.NameOf((&ProcessStep{Logger: zap.NewExample().Sugar(), manual: true}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
