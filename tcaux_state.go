package towercontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// TCAUXState contains the latest state for a fixture
type TCAUXState struct {
	operational *tcauxMessage

	l      *protostream.Socket
	logger *zap.SugaredLogger
	c      <-chan *protostream.Message
	ctx    context.Context
	mx     sync.Mutex
}

type tcauxMessage struct {
	message    *tower.TauxToTower
	lastSeen   time.Time
	dataExpiry time.Duration
}

// TCAUXStateOption is an option func for configuring a FixtureState
type TCAUXStateOption func(fs *TCAUXState)

// NewTCAUXState returns a new pointer to a configured FixtureState
// fs.Run will PANIC if WithListener is not passed as an option
func NewTCAUXState(options ...TCAUXStateOption) *TCAUXState {
	ts := &TCAUXState{operational: &tcauxMessage{}}

	ts.operational.dataExpiry = _defaultDataExpiry

	for _, option := range options {
		option(ts)
	}

	if ts.logger == nil {
		ts.logger = zap.NewExample().Sugar()
	}

	if ts.ctx == nil {
		ts.ctx = context.Background()
	}

	return ts
}

// WithTCAUXListener sets the listener configuration on a TCAUXState
func WithTCAUXListener(l *protostream.Socket) TCAUXStateOption {
	return func(ts *TCAUXState) {
		ts.l = l
	}
}

// WithTCAUXLogger sets the logger configuration on a TCAUXState
func WithTCAUXLogger(logger *zap.SugaredLogger) TCAUXStateOption {
	return func(ts *TCAUXState) {
		ts.logger = logger
	}
}

// WithTCAUXContext sets the context configuration on a TCAUXState
func WithTCAUXContext(ctx context.Context) TCAUXStateOption {
	return func(ts *TCAUXState) {
		ts.ctx = ctx
	}
}

// WithTCAUXDataExpiry sets the data expiry configuration on a TCAUXState operational message
func WithTCAUXDataExpiry(expiry time.Duration) TCAUXStateOption {
	return func(ts *TCAUXState) {
		ts.operational.dataExpiry = expiry
	}
}

// RunNewTCAUXState creates a new TCAUXState and runs the updates in the background.
func RunNewTCAUXState(options ...TCAUXStateOption) *TCAUXState {
	ts := NewTCAUXState(options...)
	ts.Run(ts.ctx)

	return ts
}

// Run runs the TCAUXState updater as a NON-BLOCKING call
func (ts *TCAUXState) Run(ctx context.Context) {
	ts.logger.Info("running tcaux listener")

	go func() {
		ts.mx.Lock() // only one instance of this function allowed at a time
		defer ts.mx.Unlock()

		ts.c = ts.l.AlwaysListen()
		defer ts.l.Quit()

		for {
			select {
			case msg := <-ts.c:
				if err := ts.update(msg); err != nil {
					ts.logger.Warnw("TCAUXState run update", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (ts *TCAUXState) update(msg *protostream.Message) error {
	t2t, err := unmarshalTCAUXProtoMessage(msg)
	if err != nil {
		return fmt.Errorf("unmarshal protostream.Message: %v", err)
	}

	op := t2t.GetOp()
	if op == nil {
		return nil
	}

	ts.operational.message = t2t
	ts.operational.lastSeen = time.Now()

	return nil
}

// GetOp returns the operational message or an error if it is expired
func (ts *TCAUXState) GetOp() (*tower.TauxToTower, error) {
	if time.Since(ts.operational.lastSeen) > ts.operational.dataExpiry {
		return nil, fmt.Errorf("fixture operational data stale; did not receive new message since %s, expect update within %s", ts.operational.lastSeen, ts.operational.dataExpiry)
	}

	return ts.operational.message, nil
}
