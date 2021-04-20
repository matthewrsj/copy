package cdcontroller

import (
	"sync"
	"time"
)

// PreparedForDelivery is the message sent from C/D controller to Tower Controller to reserve a fixture while a
// tray is on the way
type PreparedForDelivery struct {
	Tray    string `json:"tray"`
	Fixture string `json:"fixture"`
}

type pfdManager struct {
	// tray to tray mapping for handling two trays
	sent   map[string]time.Time
	mx     *sync.Mutex
	expiry time.Duration
}

type pfdMOption func(*pfdManager)

func withPFDExpiry(t time.Duration) pfdMOption {
	return func(p *pfdManager) {
		p.expiry = t
	}
}

const _pfdMExpiryDefault = time.Second * 5

// newPFDManager returns a properly-instantiated *pfdManager
func newPFDManager(opts ...pfdMOption) *pfdManager {
	p := &pfdManager{
		sent:   make(map[string]time.Time),
		mx:     &sync.Mutex{},
		expiry: _pfdMExpiryDefault,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *pfdManager) sentFor(aisle string) {
	p.mx.Lock()
	defer p.mx.Unlock()

	p.sent[aisle] = time.Now()
}

func (p *pfdManager) clearFor(aisle string) {
	p.mx.Lock()
	defer p.mx.Unlock()

	delete(p.sent, aisle)
}

func (p *pfdManager) has(aisle string) bool {
	t, ok := p.sent[aisle]

	if ok && time.Since(t) > p.expiry {
		p.clearFor(aisle)

		ok = false
	}

	return ok
}
