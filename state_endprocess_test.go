package towercontroller

import (
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestEndProcess_Action(t *testing.T) {
	sc := make(chan *protostream.Message)
	exp := 1
	ep := &EndProcess{
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		childLogger: zap.NewExample().Sugar(),
		Config: Configuration{
			CellMap: map[string][]string{
				"A": {"A01", "A02"},
			},
		},
		CellAPIClient: cellapi.NewClient(""),
		cellResponse: []*pb.Cell{
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cellapi.CellData{
			"A01": {
				Position: "A01",
				Serial:   "A01CEREAL",
				IsEmpty:  false,
			},
			"A02": {
				Position: "A02",
				Serial:   "A02CEREAL",
				IsEmpty:  false,
			},
			"A03": {
				Position: "A03",
				Serial:   "A03CEREAL",
				IsEmpty:  false,
			},
		},
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	as := ep.Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"UpdateProcessStatus",
		func(*cellapi.Client, string, string, cellapi.TrayStatus) error {
			return nil
		},
	)
	defer ups.Unpatch()

	scs := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"SetCellStatuses",
		func(*cellapi.Client, []cellapi.CellPFData) error {
			return nil
		},
	)
	defer scs.Unpatch()

	msg, err := marshalMessage(&pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			sc <- msg
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestEndProcess_ActionBadOrientation(t *testing.T) {
	sc := make(chan *protostream.Message)
	ep := &EndProcess{
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		childLogger: zap.NewExample().Sugar(),
		Config: Configuration{
			CellMap: map[string][]string{
				"A": {"A01", "A02"},
			},
		},
		CellAPIClient: cellapi.NewClient(""),
		cellResponse: []*pb.Cell{
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cellapi.CellData{
			"A01": {
				Position: "A01",
				Serial:   "A01CEREAL",
				IsEmpty:  false,
			},
			"A02": {
				Position: "A02",
				Serial:   "A02CEREAL",
				IsEmpty:  false,
			},
			"A03": {
				Position: "A03",
				Serial:   "A03CEREAL",
				IsEmpty:  false,
			},
		},
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationB,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	as := ep.Actions()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"UpdateProcessStatus",
		func(*cellapi.Client, string, string, cellapi.TrayStatus) error {
			return assert.AnError
		},
	)
	defer ups.Unpatch()

	msg, err := marshalMessage(&pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			sc <- msg
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestEndProcess_ActionShortMap(t *testing.T) {
	sc := make(chan *protostream.Message)
	ep := &EndProcess{
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		childLogger: zap.NewExample().Sugar(),
		Config: Configuration{
			CellMap: map[string][]string{
				"A": {},
			},
		},
		CellAPIClient: cellapi.NewClient(""),
		cellResponse: []*pb.Cell{
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cellapi.CellData{
			"A01": {
				Position: "A01",
				Serial:   "A01CEREAL",
				IsEmpty:  false,
			},
			"A02": {
				Position: "A02",
				Serial:   "A02CEREAL",
				IsEmpty:  false,
			},
			"A03": {
				Position: "A03",
				Serial:   "A03CEREAL",
				IsEmpty:  false,
			},
		},
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	as := ep.Actions()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"UpdateProcessStatus",
		func(*cellapi.Client, string, string, cellapi.TrayStatus) error {
			return assert.AnError
		},
	)
	defer ups.Unpatch()

	msg, err := marshalMessage(&pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			sc <- msg
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestEndProcess_ActionBadSetCellStatus(t *testing.T) {
	sc := make(chan *protostream.Message)
	exp := 1
	ep := &EndProcess{
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		childLogger: zap.NewExample().Sugar(),
		Config: Configuration{
			CellMap: map[string][]string{
				"A": {"A01", "A02", "A03"},
			},
		},
		CellAPIClient: cellapi.NewClient(""),
		cellResponse: []*pb.Cell{
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Cellstatus: pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cellapi.CellData{
			"A01": {
				Position: "A01",
				Serial:   "A01CEREAL",
				IsEmpty:  false,
			},
			"A02": {
				Position: "A02",
				Serial:   "A02CEREAL",
				IsEmpty:  false,
			},
			"A03": {
				Position: "A03",
				Serial:   "A03CEREAL",
				IsEmpty:  false,
			},
		},
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	as := ep.Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	ups := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"UpdateProcessStatus",
		func(*cellapi.Client, string, string, cellapi.TrayStatus) error {
			return nil
		},
	)
	defer ups.Unpatch()

	scs := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cellapi.Client{}),
		"SetCellStatuses",
		func(*cellapi.Client, []cellapi.CellPFData) error {
			return assert.AnError
		},
	)
	defer scs.Unpatch()

	msg, err := marshalMessage(&pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Second)
			sc <- msg
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestEndProcess_Next(t *testing.T) {
	if n := (&EndProcess{childLogger: zap.NewExample().Sugar()}).Next(); statemachine.NameOf(n) != "Unloading" {
		t.Errorf("expected next state Unloading, got %s", statemachine.NameOf(n))
	}
}
