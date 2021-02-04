package towercontroller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

func TestNewTCAUXState(t *testing.T) {
	ps := &protostream.Socket{}
	zl := zap.NewExample().Sugar()

	// golint requires a custom string as ctx key to avoid collisions
	type customString string

	ts := NewTCAUXState(
		WithTCAUXListener(ps),
		WithTCAUXLogger(zl),
		WithTCAUXContext(context.WithValue(context.Background(), customString("test"), "testval")),
		WithTCAUXDataExpiry(time.Second*3),
	)

	assert.Equal(t, ps, ts.l)
	assert.Equal(t, zl, ts.logger)
	assert.Equal(t, ts.ctx.Value(customString("test")), "testval")
	assert.Equal(t, ts.operational.dataExpiry, time.Second*3)
}

func TestTCAUXState_update(t *testing.T) {
	ts := NewTCAUXState()

	opMsg := mustMarshalProtostreamMessage(
		t,
		&tower.TauxToTower{
			Content: &tower.TauxToTower_Op{
				Op: &tower.TauxOperational{
					PowerCapacityW:  10,
					PowerInUseW:     2,
					PowerAvailableW: 8,
				},
			},
			Versions: &tower.Versions{
				SoftwareVersion: "0.1.0",
				ProtocolVersion: 1,
			},
		},
	)

	if err := ts.update(opMsg); err != nil {
		t.Fatal(err)
	}

	result, err := ts.GetOp()
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, int32(8), result.GetOp().GetPowerAvailableW())
}
