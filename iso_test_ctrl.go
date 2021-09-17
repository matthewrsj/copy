package towercontroller

import (
	"context"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

const _pulseCycle = time.Second * 30

// TurnOnIsolationTest turns on isolation test for whole tower.
// Isolation test must be enabled on one and only one fixture at a time.
func TurnOnIsolationTest(ctx context.Context, sugar *zap.SugaredLogger, registry map[string]*FixtureInfo, publisher *protostream.Socket, on, off []string) {
	isoReqOn := &tower.TowerToFixture{IsolationTestRequest: tower.IsolationTestRequest_ISOLATION_TEST_REQUEST_ENABLE}
	isoReqOff := &tower.TowerToFixture{IsolationTestRequest: tower.IsolationTestRequest_ISOLATION_TEST_REQUEST_DISABLE}

	// initial settings
	pulseIsoTest(sugar, registry, publisher, on, off, isoReqOn, isoReqOff)

	// pulse request
	tick := time.NewTicker(_pulseCycle)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			pulseIsoTest(sugar, registry, publisher, on, off, isoReqOn, isoReqOff)
		case <-ctx.Done():
			return
		}
	}
}

func pulseIsoTest(sugar *zap.SugaredLogger, registry map[string]*FixtureInfo, publisher *protostream.Socket, on, off []string, isoReqOn, isoReqOff *tower.TowerToFixture) {
	if len(on) == 0 {
		sugar.Error("on fixture list must contain at least one element")
		return
	}

	selected := findIsoCtrlFixture(registry, on)
	if selected == "" {
		sugar.Error("unable to identify isolation control fixture")
		return
	}

	// first make sure it is disabled on all fixtures
	for i := range off {
		if off[i] == selected {
			continue
		}

		if err := sendProtoMessage(publisher, isoReqOff, off[i]); err != nil {
			sugar.Warn("turn off isolation control on fixture", "target_fixture", off[i], zap.Error(err))
		}
	}

	// then enable it on the first fixture in the AllowedFixtures list (len check done above)
	// isolation test only works when it is enabled on one and only one fixture
	if err := sendProtoMessage(publisher, isoReqOn, selected); err != nil {
		sugar.Warn("turn on isolation control on fixture", "target_fixture", on, zap.Error(err))
	}
}

func findIsoCtrlFixture(registry map[string]*FixtureInfo, pool []string) string {
	if len(pool) == 0 {
		return ""
	}

	selected := pool[0]

	for i := range pool {
		fxr, ok := registry[pool[i]]
		if !ok {
			continue
		}

		if _, err := fxr.FixtureState.GetOp(); err == nil {
			// found one that is transmitting
			selected = pool[i]
			break
		}
	}

	return selected
}
