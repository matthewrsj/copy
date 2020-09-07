package towercontroller

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"nanomsg.org/go/mangos/v2"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

func TestNewFixtureState(t *testing.T) {
	ps := &protostream.Socket{}
	zl := zap.NewExample().Sugar()

	// golint requires a custom string as ctx key to avoid collisions
	type customString string

	fs := NewFixtureState(
		WithListener(ps),
		WithLogger(zl),
		WithContext(context.WithValue(context.Background(), customString("test"), "testval")),
		WithAllDataExpiry(time.Second*3), // should be overwritten by below options
		WithOperationalDataExpiry(time.Second),
		WithDiagnosticDataExpiry(time.Second*2),
	)

	assert.Equal(t, ps, fs.l)
	assert.Equal(t, zl, fs.logger)
	assert.Equal(t, fs.ctx.Value(customString("test")), "testval")
	assert.Equal(t, fs.operational.dataExpiry, time.Second)
	assert.Equal(t, fs.diagnostic.dataExpiry, time.Second*2)
}

func TestFixtureState_update(t *testing.T) {
	fs := NewFixtureState()

	msg := mustMarshalProtostreamMessage(t, &pb.FixtureToTower{Fixturebarcode: "test"})
	if err := fs.update(msg); err == nil {
		t.Error("no error returned for unknown type")
	}

	opMsg := mustMarshalProtostreamMessage(
		t,
		&pb.FixtureToTower{
			Content: &pb.FixtureToTower_Op{
				Op: &pb.FixtureOperational{},
			},
			Fixturebarcode: "test",
		},
	)

	diagMsg := mustMarshalProtostreamMessage(
		t,
		&pb.FixtureToTower{
			Content: &pb.FixtureToTower_Diag{
				Diag: &pb.FixtureDiagnostic{},
			},
			Fixturebarcode: "test",
		},
	)

	alertMsg := mustMarshalProtostreamMessage(
		t,
		&pb.FixtureToTower{
			Content: &pb.FixtureToTower_AlertLog{
				AlertLog: &pb.AlertLog{},
			},
			Fixturebarcode: "test",
		},
	)

	for _, msg := range []*protostream.Message{opMsg, diagMsg, alertMsg} {
		if err := fs.update(msg); err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range []func() (*pb.FixtureToTower, error){fs.GetOp, fs.GetDiag, fs.GetAlert} {
		msg, err := f()
		if err != nil {
			t.Error(err)
			continue
		}

		assert.Equal(t, "test", msg.Fixturebarcode)
	}
}

func TestFixtureState_GetAlertNil(t *testing.T) {
	fs := NewFixtureState()
	if _, err := fs.GetAlert(); err == nil {
		t.Error("got no error when no alert was present")
	}
}

func TestFixtureState_getInternal(t *testing.T) {
	fs := NewFixtureState()
	fs.operational.message = &pb.FixtureToTower{Fixturebarcode: "test"}

	if _, err := getInternal(fs.operational); err == nil {
		t.Error("got no error when data expiry was violated")
	}

	fs.operational.lastSeen = time.Now()

	op, err := getInternal(fs.operational)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "test", op.Fixturebarcode)
}

func mustMarshalProtostreamMessage(t *testing.T, msg proto.Message) *protostream.Message {
	t.Helper()

	protob, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	ev := protostream.ProtoMessage{
		Location: "01-01",
		Body:     protob,
	}

	evb, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	return &protostream.Message{
		Msg: &mangos.Message{
			Body: evb,
		},
	}
}
