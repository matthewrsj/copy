package towercontroller

import (
	"reflect"
	"testing"

	"bou.ke/monkey"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestStartProcess_Action(t *testing.T) {
	sc := make(chan *protostream.Message)
	cmc := make([]string, 64)
	cmc[0] = "A01"
	cmc[1] = "A02"
	spState := StartProcess{
		SubscribeChan: sc,
		Publisher:     &protostream.Socket{},
		Config: Configuration{
			CellMap: map[string][]string{
				"A": cmc,
			},
			AllowedFixtures: []string{"01-01"},
		},
		childLogger:     zap.NewExample().Sugar(),
		CellAPIClient:   &cellapi.Client{},
		processStepName: "",
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationA,
			Raw: "11223344A",
		},
		fxbc: traycontrollers.FixtureBarcode{
			Tower: "01",
			Fxn:   "01",
		},
		steps: traycontrollers.StepConfiguration{{Mode: "test"}},
	}
	as := (&spState).Actions()

	exp := 1
	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	gcmp := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"GetCellMap",
		func(_ *cellapi.Client, sn string) (map[string]cellapi.CellData, error) {
			return map[string]cellapi.CellData{
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

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"UpdateProcessStatus",
		func(*cellapi.Client, string, string, cellapi.TrayStatus) error { return nil },
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

	msg, err := marshalMessage(&pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Status: pb.FixtureStatus_FIXTURE_STATUS_READY,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 2; i++ {
			sc <- msg
		}
	}()

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
