package towercontroller

import (
	"sync"

	"stash.teslamotors.com/rr/protostream"
	"stash.teslamotors.com/rr/traycontrollers"
)

// FixtureInfo contains the feeds for messages from the C/D Controller
type FixtureInfo struct {
	Name      string
	PFD       chan traycontrollers.PreparedForDelivery
	LDC       chan traycontrollers.FXRLoad
	SC        <-chan *protostream.Message
	Unreserve chan struct{}
	Avail     ReadyStatus
}

// ReadyStatus indicates the status of the fixture
type ReadyStatus struct {
	ready Status
	mx    sync.Mutex
}

// Set sets the status of the fixture to the ready argument
func (r *ReadyStatus) Set(ready Status) {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.ready = ready
}

// Status returns the internal status of the fixture
func (r *ReadyStatus) Status() Status {
	return r.ready
}

// Status is the status of the fixture
type Status int

// Status defaults
const (
	StatusUnknown Status = iota
	StatusWaitingForReservation
	StatusWaitingForLoad
	StatusActive
	StatusUnloading
)
