package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestIdle_Next(t *testing.T) {
	i := &Idle{
		Logger: zap.NewExample().Sugar(),
		FXRInfo: &FixtureInfo{
			Name: "01-01",
		},
		Config: Configuration{
			AllowedFixtures: []string{"01-01"},
		},
		next: &WaitForLoad{},
	}
	assert.Equal(t, "WaitForLoad", statemachine.NameOf(i.Next()))
}

func TestIdle_NextErr(t *testing.T) {
	i := &Idle{
		Logger:  zap.NewExample().Sugar(),
		FXRInfo: &FixtureInfo{},
		err:     assert.AnError,
	}
	assert.Equal(t, "Idle", statemachine.NameOf(i.Next()))
}

func TestIdle_Actions(t *testing.T) {
	pfdC := make(chan cdcontroller.PreparedForDelivery)
	i := &Idle{
		Config: Configuration{
			AllowedFixtures: []string{"01-01"},
		},
		Logger: zap.NewExample().Sugar(),
		FXRInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
			Name:         "01-01",
			PFD:          pfdC,
		},
	}

	updateInternalFixtureState(i.FXRInfo.FixtureState.operational, &pb.FixtureToTower{})

	as := i.Actions()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when running action: %v", r)
		}
	}()

	go func() {
		pfdC <- cdcontroller.PreparedForDelivery{
			Tray:    "11223344A",
			Fixture: "CM2-63010-01-01",
		}
	}()

	for _, a := range as {
		a()
	}

	assert.Equal(t, "WaitForLoad", statemachine.NameOf(i.Next()))
}

func TestIdle_ActionsBadTray(t *testing.T) {
	pfdC := make(chan cdcontroller.PreparedForDelivery)
	i := &Idle{
		Config: Configuration{
			AllowedFixtures: []string{"01-01"},
		},
		Logger: zap.NewExample().Sugar(),
		FXRInfo: &FixtureInfo{
			PFD: pfdC,
		},
	}

	as := i.Actions()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when running action: %v", r)
		}
	}()

	go func() {
		pfdC <- cdcontroller.PreparedForDelivery{
			Tray:    "11223",
			Fixture: "CM2-63010-01-01",
		}
	}()

	for _, a := range as {
		a()
	}

	assert.Equal(t, "Idle", statemachine.NameOf(i.Next()))
}
