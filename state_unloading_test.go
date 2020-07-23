package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestUnloading_Next(t *testing.T) {
	assert.Equal(t, "Idle", statemachine.NameOf((&Unloading{childLogger: zap.NewExample().Sugar()}).Next()))
}

func TestUnloading_Actions(t *testing.T) {
	sc := make(chan *protostream.Message)
	ul := &Unloading{
		SubscribeChan: sc,
		Config: Configuration{
			AllowedFixtures: []string{"01-01"},
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

	msg, err := marshalMessage(
		&pb.FixtureToTower{
			Content: &pb.FixtureToTower_Op{
				Op: &pb.FixtureOperational{Status: pb.FixtureStatus_FIXTURE_STATUS_IDLE},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		sc <- msg
		close(sc)
	}()

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
