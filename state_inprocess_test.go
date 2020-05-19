package towercontroller

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"
	pb "stash.teslamotors.com/rr/towerproto"

	"bou.ke/monkey"
	"github.com/linklayer/go-socketcan/pkg/socketcan"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

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
	exp := 1
	ipState := &InProcess{
		Config: Configuration{
			Fixtures: map[string]uint32{
				"01": 1,
			},
		},
		Logger: logrus.New(),
		tbc: TrayBarcode{
			SN:  "11223344",
			O:   _orientA,
			raw: "11223344A",
		},
		fxbc: FixtureBarcode{
			Location: "SWIFT",
			Aisle:    "01",
			Tower:    "A",
			Fxn:      "01",
			raw:      "SWIFT-01-A-01",
		},
	}
	as := ipState.Actions()

	if l := len(as); l != exp {
		t.Errorf("expected %d actions, got %d", exp, l)
	}

	ifp := monkey.Patch(socketcan.NewIsotpInterface, patchNewIsotpInterface)
	defer ifp.Unpatch()

	rbp := monkey.PatchInstanceMethod(
		reflect.TypeOf(socketcan.Interface{}),
		"RecvBuf",
		patchRecvBuffFunc(&pb.FixtureToTower{
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
			Traybarcode:     "",
			Fixtureposition: ipState.fxbc.raw,
		}),
	)
	defer rbp.Unpatch()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic when actions called: %v", r)
		}
	}()

	for _, a := range as {
		a() // if a panic occurs it will be caught by the deferred func
	}
}

func TestInProcess_ActionNoFixture(t *testing.T) {
	ipState := InProcess{
		Logger: logrus.New(),
	}
	as := ipState.Actions()

	for _, a := range as {
		a()
	}

	assert.True(t, ipState.Last())
}

func TestInProcess_ActionNoIface(t *testing.T) {
	ipState := InProcess{
		Config: Configuration{
			Fixtures: map[string]uint32{
				"01": 1,
			},
		},
		Logger: logrus.New(),
		fxbc: FixtureBarcode{
			Location: "SWIFT",
			Aisle:    "01",
			Tower:    "A",
			Fxn:      "01",
			raw:      "SWIFT-01-A-01",
		},
	}

	ifp := monkey.Patch(socketcan.NewIsotpInterface, func(string, uint32, uint32) (socketcan.Interface, error) {
		return socketcan.Interface{}, assert.AnError
	})
	defer ifp.Unpatch()

	as := ipState.Actions()

	for _, a := range as {
		a()
	}

	assert.True(t, ipState.Last())
}

func TestInProcess_ActionRecvBufErr(t *testing.T) {
	ipState := InProcess{
		Config: Configuration{
			Fixtures: map[string]uint32{
				"01": 1,
			},
		},
		Logger: logrus.New(),
		fxbc: FixtureBarcode{
			Location: "SWIFT",
			Aisle:    "01",
			Tower:    "A",
			Fxn:      "01",
			raw:      "SWIFT-01-A-01",
		},
	}

	ifp := monkey.Patch(socketcan.NewIsotpInterface, patchNewIsotpInterface)
	defer ifp.Unpatch()

	rbp := monkey.PatchInstanceMethod(
		reflect.TypeOf(socketcan.Interface{}),
		"RecvBuf",
		func(socketcan.Interface) ([]byte, error) {
			return nil, assert.AnError
		},
	)
	rbp.Unpatch()

	as := ipState.Actions()

	for _, a := range as {
		a()
	}

	assert.True(t, ipState.Last())
}

func TestInProcess_ActionBadBuffer(t *testing.T) {
	ipState := InProcess{
		Config: Configuration{
			Fixtures: map[string]uint32{
				"01": 1,
			},
		},
		Logger: logrus.New(),
		fxbc: FixtureBarcode{
			Location: "SWIFT",
			Aisle:    "01",
			Tower:    "A",
			Fxn:      "01",
			raw:      "SWIFT-01-A-01",
		},
	}

	ifp := monkey.Patch(socketcan.NewIsotpInterface, patchNewIsotpInterface)
	defer ifp.Unpatch()

	rbp := monkey.PatchInstanceMethod(
		reflect.TypeOf(socketcan.Interface{}),
		"RecvBuf",
		func(socketcan.Interface) ([]byte, error) {
			return []byte("this is not proto"), nil
		},
	)
	defer rbp.Unpatch()

	as := ipState.Actions()

	for _, a := range as {
		a()
	}

	assert.True(t, ipState.Last())
}

func TestInProcess_Next(t *testing.T) {
	exp := "EndProcess"
	if n := statemachine.NameOf((&InProcess{Logger: logrus.New()}).Next()); n != exp {
		t.Errorf("expected next state name to be %s, got %s", exp, n)
	}
}
