package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestWaitForLoad_Next(t *testing.T) {
	assert.Equal(t, "ProcessStep", statemachine.NameOf((&WaitForLoad{Logger: zap.NewExample().Sugar()}).Next()))
}

func TestWaitForLoad_Actions(t *testing.T) {
	lc := make(chan traycontrollers.FXRLoad)
	wfl := &WaitForLoad{
		Config: Configuration{
			Loc: location{
				Line:    "CM2",
				Process: "63",
				Aisle:   "010",
			},
			Fixtures: map[string]fixtureConf{
				"01-01": {
					Bus: "vcan0",
					RX:  0x1c1,
					TX:  0x241,
				},
			},
		},
		Logger: zap.NewExample().Sugar(),
		fxrInfo: &FixtureInfo{
			LDC: lc,
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic while running action: %v", r)
		}
	}()

	go func() {
		lc <- traycontrollers.FXRLoad{
			Column:        1,
			Level:         1,
			TrayID:        "11223344A",
			RecipeName:    "test",
			RecipeVersion: 1,
		}
	}()

	as := wfl.Actions()

	for _, a := range as {
		a()
	}

	assert.Equal(t, "test", wfl.processStepName)
}
