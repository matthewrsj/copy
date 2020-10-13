package towercontroller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

const _defaultDataExpiry = time.Second * 10

// FixtureState contains the latest state for a fixture
type FixtureState struct {
	operational *fixtureMessage
	diagnostic  *fixtureMessage
	alert       *fixtureMessage

	l      *protostream.Socket
	logger *zap.SugaredLogger
	c      <-chan *protostream.Message
	ctx    context.Context
	mx     sync.Mutex
}

type fixtureMessage struct {
	message    *tower.FixtureToTower
	lastSeen   time.Time
	dataExpiry time.Duration
}

// FixtureStateOption is an option func for configuring a FixtureState
type FixtureStateOption func(fs *FixtureState)

// NewFixtureState returns a new pointer to a configured FixtureState
// fs.Run will PANIC if WithListener is not passed as an option
func NewFixtureState(options ...FixtureStateOption) *FixtureState {
	fs := &FixtureState{
		operational: &fixtureMessage{},
		diagnostic:  &fixtureMessage{},
		alert:       &fixtureMessage{},
	}

	fs.operational.dataExpiry = _defaultDataExpiry
	fs.diagnostic.dataExpiry = _defaultDataExpiry

	for _, option := range options {
		option(fs)
	}

	if fs.logger == nil {
		fs.logger = zap.NewExample().Sugar()
	}

	if fs.ctx == nil {
		fs.ctx = context.Background()
	}

	return fs
}

// WithListener sets the listener configuration on a FixtureState
func WithListener(l *protostream.Socket) FixtureStateOption {
	return func(fs *FixtureState) {
		fs.l = l
	}
}

// WithLogger sets the logger configuration on a FixtureState
func WithLogger(logger *zap.SugaredLogger) FixtureStateOption {
	return func(fs *FixtureState) {
		fs.logger = logger
	}
}

// WithContext sets the context configuration on a FixtureState
func WithContext(ctx context.Context) FixtureStateOption {
	return func(fs *FixtureState) {
		fs.ctx = ctx
	}
}

// WithOperationalDataExpiry sets the data expiry configuration on a FixtureState operational message
func WithOperationalDataExpiry(expiry time.Duration) FixtureStateOption {
	return func(fs *FixtureState) {
		setExpiry(fs.operational, expiry)
	}
}

// WithDiagnosticDataExpiry sets the data expiry configuration on a FixtureState operational message
func WithDiagnosticDataExpiry(expiry time.Duration) FixtureStateOption {
	return func(fs *FixtureState) {
		setExpiry(fs.diagnostic, expiry)
	}
}

// WithAllDataExpiry sets the data expiry on operational and diagnostic messages
func WithAllDataExpiry(expiry time.Duration) FixtureStateOption {
	return func(fs *FixtureState) {
		for _, internal := range []*fixtureMessage{fs.operational, fs.diagnostic} { // alerts do not invalidate
			setExpiry(internal, expiry)
		}
	}
}

// RunNewFixtureState creates a new FixtureState and runs the updates in the background.
func RunNewFixtureState(options ...FixtureStateOption) *FixtureState {
	fs := NewFixtureState(options...)
	fs.Run(fs.ctx)

	return fs
}

// Run runs the FixtureState updater as a NON-BLOCKING call
func (fs *FixtureState) Run(ctx context.Context) {
	go func() {
		fs.mx.Lock() // only one instance of this function allowed at a time
		defer fs.mx.Unlock()

		fs.c = fs.l.AlwaysListen()
		defer fs.l.Quit()

		for {
			select {
			case msg := <-fs.c:
				if err := fs.update(msg); err != nil {
					fs.logger.Warnw("FixtureState run update", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (fs *FixtureState) update(msg *protostream.Message) error {
	f2t, err := unmarshalProtoMessage(msg)
	if err != nil {
		return fmt.Errorf("unmarshal protostream.Message: %v", err)
	}

	switch f2t.Content.(type) {
	case *tower.FixtureToTower_Op:
		updateInternalFixtureState(fs.operational, f2t)
	case *tower.FixtureToTower_Diag:
		updateInternalFixtureState(fs.diagnostic, f2t)
	case *tower.FixtureToTower_AlertLog:
		updateInternalFixtureState(fs.alert, f2t)
	default:
		return fmt.Errorf("received unknown type: %T", f2t.Content)
	}

	return nil
}

func updateInternalFixtureState(internal *fixtureMessage, msg *tower.FixtureToTower) {
	internal.message = msg
	internal.lastSeen = time.Now()
}

func setExpiry(internal *fixtureMessage, expiry time.Duration) {
	internal.dataExpiry = expiry
}

// GetOp returns the operational message or an error if it is expired
func (fs *FixtureState) GetOp() (*tower.FixtureToTower, error) {
	return getInternal(fs.operational)
}

// GetDiag returns the diagnostic message or an error if it is expired
func (fs *FixtureState) GetDiag() (*tower.FixtureToTower, error) {
	return getInternal(fs.diagnostic)
}

// GetAlert returns the latest alert message. Will return an error if no alerts have ever been received,
// but will not return an error due to any data expiry.
func (fs *FixtureState) GetAlert() (*tower.FixtureToTower, error) {
	if fs.alert.message == nil {
		return nil, errors.New("no alerts have been received")
	}

	return fs.alert.message, nil
}

func getInternal(internal *fixtureMessage) (*tower.FixtureToTower, error) {
	if time.Since(internal.lastSeen) > internal.dataExpiry {
		return nil, fmt.Errorf("fixture operational data stale; did not receive new message since %s, expect update within %s", internal.lastSeen, internal.dataExpiry)
	}

	return internal.message, nil
}
