package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestUnloading_Next(t *testing.T) {
	assert.Equal(t, "Idle", statemachine.NameOf((&Unloading{Logger: zap.NewExample().Sugar()}).Next()))
}

func TestUnloading_Actions(t *testing.T) {
	ul := &Unloading{
		Config: Configuration{
			Fixtures: map[string]uint32{
				"01-01": 1,
			},
		},
		Logger: zap.NewExample().Sugar(),
		fxbc: traycontrollers.FixtureBarcode{
			Location: "",
			Aisle:    "",
			Tower:    "01",
			Fxn:      "01",
			Raw:      "01",
		},
		fxrInfo: &FixtureInfo{},
	}

	as := ul.Actions()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic while running action: %v", r)
		}
	}()

	for _, a := range as {
		a()
	}
}
