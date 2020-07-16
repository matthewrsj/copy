package towercontroller

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

func TestMain(m *testing.M) {
	pni := monkey.Patch(socketcan.NewIsotpInterface, patchNewIsotpInterface)
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
	psc := monkey.PatchInstanceMethod(reflect.TypeOf(socketcan.Interface{}),
		"SetCANFD",
		func(socketcan.Interface) error { return nil },
	)

	ret := m.Run()

	pni.Unpatch()
	prb.Unpatch()
	psc.Unpatch()

	os.Exit(ret)
}

func patchNewIsotpInterface(dev string, rxid, txid uint32) (socketcan.Interface, error) {
	return socketcan.Interface{
		IfName:   dev,
		SocketFd: 0,
	}, nil
}

func patchRecvBuffFunc(msg proto.Message) func(socketcan.Interface) ([]byte, error) {
	buf, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}

	return func(socketcan.Interface) ([]byte, error) {
		return buf, nil
	}
}

func TestInProcess_Action(t *testing.T) {
	sc := make(chan *protostream.Message)
	exp := 1
	ipState := &InProcess{
		SubscribeChan: sc,
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
		tbc: traycontrollers.TrayBarcode{
			SN:  "11223344",
			O:   traycontrollers.OrientationA,
			Raw: "11223344A",
		},
		fxbc: traycontrollers.FixtureBarcode{
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

	msg := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Status: pb.FixtureStatus_FIXTURE_STATUS_COMPLETE,
				Cells: []*pb.Cell{
					{
						Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
						Cellmeasurement: &pb.CellMeasurement{
							Current: 3.49,
						},
					},
					{
						Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
						Cellmeasurement: &pb.CellMeasurement{
							Current: 3.49,
						},
					},
				},
			},
		},
		Traybarcode:    "",
		Fixturebarcode: ipState.fxbc.Raw,
	}

	event, err := marshalMessage(msg)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		sc <- event
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
		childLogger:   zap.NewExample().Sugar(),
		SubscribeChan: sc,
	}
	as := ipState.Actions()

	msg, err := marshalMessage(&pb.FixtureToTower{})
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
