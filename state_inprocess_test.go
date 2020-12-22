package towercontroller

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"bou.ke/monkey"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

func TestInProcess_Action(t *testing.T) {
	exp := 1
	ipState := &InProcess{
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
		Config: Configuration{
			AllowedFixtures: []string{"01-01"},
		},
		childLogger: zap.NewExample().Sugar(),
		tbc: cdcontroller.TrayBarcode{
			SN:  "11223344",
			O:   cdcontroller.OrientationA,
			Raw: "11223344A",
		},
		fxbc: cdcontroller.FixtureBarcode{
			Location: "CM2",
			Aisle:    "63010",
			Tower:    "01",
			Fxn:      "01",
			Raw:      "CM2-63010-01-01",
		},
	}
	as := ipState.Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	hp := monkey.PatchInstanceMethod(reflect.TypeOf(&http.Client{}), "Post", func(*http.Client, string, string, io.Reader) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(strings.NewReader(""))}, nil
	})
	defer hp.Unpatch()

	updateInternalFixtureState(
		ipState.fxrInfo.FixtureState.operational,
		&tower.FixtureToTower{
			Content: &tower.FixtureToTower_Op{
				Op: &tower.FixtureOperational{
					Status: tower.FixtureStatus_FIXTURE_STATUS_COMPLETE,
					Cells: []*tower.Cell{
						{
							Status: tower.CellStatus_CELL_STATUS_COMPLETE,
							Measurement: &tower.CellMeasurement{
								Current: 3.49,
							},
						},
						{
							Status: tower.CellStatus_CELL_STATUS_COMPLETE,
							Measurement: &tower.CellMeasurement{
								Current: 3.49,
							},
						},
					},
				},
			},
			Info: &tower.Info{
				TrayBarcode:     "",
				FixtureLocation: ipState.fxbc.Raw,
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

func marshalMessage(msg protoreflect.ProtoMessage) (*protostream.Message, error) {
	msgb, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	pmb, err := json.Marshal(&protostream.ProtoMessage{Location: "01-02", Body: msgb})
	if err != nil {
		return nil, err
	}

	return &protostream.Message{
		Msg: &mangos.Message{
			Body: pmb,
		},
	}, nil
}

func TestInProcess_ActionNoFixture(t *testing.T) {
	sc := make(chan *protostream.Message)

	ipState := InProcess{
		childLogger: zap.NewExample().Sugar(),
		fxrInfo: &FixtureInfo{
			FixtureState: NewFixtureState(),
		},
	}
	as := ipState.Actions()

	updateInternalFixtureState(
		ipState.fxrInfo.FixtureState.operational,
		&tower.FixtureToTower{
			Content: &tower.FixtureToTower_Op{
				Op: &tower.FixtureOperational{
					Status: tower.FixtureStatus_FIXTURE_STATUS_COMPLETE,
				},
			},
		},
	)

	hp := monkey.PatchInstanceMethod(reflect.TypeOf(&http.Client{}), "Post", func(*http.Client, string, string, io.Reader) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(strings.NewReader(""))}, nil
	})
	defer hp.Unpatch()

	msg, err := marshalMessage(&tower.FixtureToTower{})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		sc <- msg
		close(sc)
	}()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	for _, a := range as {
		a()
	}
}

func TestInProcess_Next(t *testing.T) {
	exp := "EndProcess"
	if n := statemachine.NameOf((&InProcess{childLogger: zap.NewExample().Sugar()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
