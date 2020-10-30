package towercontroller

import (
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

func TestEndProcess_Action(t *testing.T) {
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
		CellAPIClient: cdcontroller.NewCellAPIClient(""),
		cellResponse: []*tower.Cell{
			{
				Status: tower.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cdcontroller.CellData{
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
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"UpdateProcessStatus",
		func(*cdcontroller.CellAPIClient, string, string, cdcontroller.TrayStatus) error {
			return nil
		},
	)
	defer ups.Unpatch()

	scs := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"SetCellStatuses",
		func(*cdcontroller.CellAPIClient, string, string, string, int, []cdcontroller.CellPFData) error {
			return nil
		},
	)
	defer scs.Unpatch()

	postP := monkey.Patch(http.Post, func(_, _ string, r io.Reader) (*http.Response, error) {
		zap.NewExample().Sugar().Info("i was called :)")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(r),
		}, nil
	})
	defer postP.Unpatch()

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
		CellAPIClient: cdcontroller.NewCellAPIClient(""),
		cellResponse: []*tower.Cell{
			{
				Status: tower.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cdcontroller.CellData{
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
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationB,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"UpdateProcessStatus",
		func(*cdcontroller.CellAPIClient, string, string, cdcontroller.TrayStatus) error {
			return assert.AnError
		},
	)
	defer ups.Unpatch()

	postP := monkey.Patch(http.Post, func(string, string, io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})
	defer postP.Unpatch()

	msg, err := marshalMessage(&tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		CellAPIClient: cdcontroller.NewCellAPIClient(""),
		cellResponse: []*tower.Cell{
			{
				Status: tower.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cdcontroller.CellData{
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
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"UpdateProcessStatus",
		func(*cdcontroller.CellAPIClient, string, string, cdcontroller.TrayStatus) error {
			return assert.AnError
		},
	)
	defer ups.Unpatch()

	postP := monkey.Patch(http.Post, func(string, string, io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})
	defer postP.Unpatch()

	msg, err := marshalMessage(&tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		CellAPIClient: cdcontroller.NewCellAPIClient(""),
		cellResponse: []*tower.Cell{
			{
				Status: tower.CellStatus_CELL_STATUS_COMPLETE,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_FAILED,
			},
			{
				Status: tower.CellStatus_CELL_STATUS_NONE_UNSPECIFIED,
			},
		},
		cells: map[string]cdcontroller.CellData{
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
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationA,
			Raw: "11223344A",
		},
		fixtureFault: true,
	}

	updateInternalFixtureState(ep.fxrInfo.FixtureState.operational, &tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"UpdateProcessStatus",
		func(*cdcontroller.CellAPIClient, string, string, cdcontroller.TrayStatus) error {
			return nil
		},
	)
	defer ups.Unpatch()

	scs := monkey.PatchInstanceMethod(
		reflect.TypeOf(&cdcontroller.CellAPIClient{}),
		"SetCellStatuses",
		func(*cdcontroller.CellAPIClient, string, string, string, int, []cdcontroller.CellPFData) error {
			return assert.AnError
		},
	)
	defer scs.Unpatch()

	postP := monkey.Patch(http.Post, func(string, string, io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader("")),
		}, nil
	})
	defer postP.Unpatch()

	msg, err := marshalMessage(&tower.FixtureToTower{
		Content: &tower.FixtureToTower_Op{
			Op: &tower.FixtureOperational{
				Position: tower.FixturePosition_FIXTURE_POSITION_OPEN,
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
