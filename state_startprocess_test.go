package towercontroller

import (
	"reflect"
	"testing"

	"bou.ke/monkey"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestStartProcess_Action(t *testing.T) {
	cmc := make([]string, 64)
	cmc[0] = "A01"
	cmc[1] = "A02"
	spState := StartProcess{
		Logger: zap.NewExample().Sugar(),
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		Publisher: &protostream.Socket{},
		Config: Configuration{
			CellMap: map[string][]string{
				"A": cmc,
			},
			AllowedFixtures: []string{"01-01"},
		},
		childLogger:     zap.NewExample().Sugar(),
		CellAPIClient:   &cdcontroller.CellAPIClient{},
		processStepName: "test",
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationA,
			Raw: "11223344A",
		},
		fxbc: cdcontroller.FixtureBarcode{
			Tower: "01",
			Fxn:   "01",
			Raw:   "CM2-63010-01-01",
		},
		steps: cdcontroller.StepConfiguration{{Mode: "test"}},
	}
	as := (&spState).Actions()

	exp := 1
	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	gcmp := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"GetCellMap",
		func(_ *cdcontroller.CellAPIClient, sn string) (map[string]cdcontroller.CellData, error) {
			return map[string]cdcontroller.CellData{
				"A01": {
					Position: "A01",
					Serial:   "TESTA01",
				},
				"A02": {
					Position: "A02",
					Serial:   "TESTA02",
				},
			}, nil
		},
	)
	defer gcmp.Unpatch()

	pio := monkey.Patch(sendProtoMessage, func(_ *protostream.Socket, _ proto.Message, _ string) error {
		return nil
	})
	defer pio.Unpatch()

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"UpdateProcessStatus",
		func(*cdcontroller.CellAPIClient, string, string, cdcontroller.TrayStatus) error { return nil },
	)
	defer ups.Unpatch()

	pub := monkey.PatchInstanceMethod(
		reflect.TypeOf(&protostream.Socket{}),
		"PublishTo",
		func(*protostream.Socket, string, []byte) error {
			return nil
		},
	)
	defer pub.Unpatch()

	updateInternalFixtureState(
		spState.fxrInfo.FixtureState.operational,
		&pb.FixtureToTower{
			Content: &pb.FixtureToTower_Op{
				Op: &pb.FixtureOperational{
					Status: pb.FixtureStatus_FIXTURE_STATUS_READY,
				},
			},
		},
	)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestStartProcess_FatalNext(t *testing.T) {
	exp := "Idle"
	if n := statemachine.NameOf((&StartProcess{childLogger: zap.NewExample().Sugar(), smFatal: true}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}

func TestStartProcess_Next(t *testing.T) {
	exp := "InProcess"
	if n := statemachine.NameOf((&StartProcess{childLogger: zap.NewExample().Sugar()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
