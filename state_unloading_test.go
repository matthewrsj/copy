package towercontroller

import (
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/ctet/statemachine/v2"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestUnloading_Next(t *testing.T) {
	assert.Equal(t, "Idle", statemachine.NameOf((&Unloading{childLogger: zap.NewExample().Sugar()}).Next()))
}

func TestUnloading_Actions(t *testing.T) {
	ul := &Unloading{
		Config: Configuration{
			Fixtures: map[string]fixtureConf{
				"01-01": {
					Bus: "vcan0",
					RX:  0x1c1,
					TX:  0x241,
				},
			},
		},
		childLogger: zap.NewExample().Sugar(),
		fxbc: traycontrollers.FixtureBarcode{
			Location: "",
			Aisle:    "",
			Tower:    "01",
			Fxn:      "01",
			Raw:      "01",
		},
		fxrInfo: &FixtureInfo{},
	}

	prb := monkey.PatchInstanceMethod(
		reflect.TypeOf(socketcan.Interface{}),
		"RecvBuf",
		patchRecvBuffFunc(
			&pb.FixtureToTower{
				Content: &pb.FixtureToTower_Op{
					Op: &pb.FixtureOperational{
						Status: pb.FixtureStatus_FIXTURE_STATUS_IDLE,
					},
				},
			},
		),
	)
	defer prb.Unpatch()

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
