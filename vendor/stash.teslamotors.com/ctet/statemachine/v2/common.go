package statemachine

import (
	"sync"
)

// Common state is an embeddable state that performs common state actions.
// These functions may be overwritten by the emedding state. All setter
// functions are thread-safe through the use of mutex.
type Common struct {
	name    string
	fatal   bool
	isLast  bool
	mx      sync.Mutex
	context interface{}
}

// Name placeholder implements the State interface
// this function should be usable as-is
func (c *Common) Name() string {
	return c.name
}

// SetName placeholder implements the State interface
// this function should be usable as-is
func (c *Common) SetName(name string) {
	c.mx.Lock()
	c.name = name
	c.mx.Unlock()
}

// Actions placeholder implements the State interface
func (c *Common) Actions() []func() {
	return []func(){
		func() {},
	}
}

// Next placeholder implements the State interface
func (c *Common) Next() State {
	return c
}

// Last placeholder implements the State interface
// this function should be usable as-is
func (c *Common) Last() bool {
	return c.isLast
}

// SetLast placeholder implements the State interface
// this function should be usable as-is
func (c *Common) SetLast(last bool) {
	c.mx.Lock()
	c.isLast = last
	c.mx.Unlock()
}

// Fatal placeholder implements the State interface
// this function should be usable as-is
func (c *Common) Fatal() bool {
	return c.fatal
}

// SetFatal placeholder implements the State interface
// this function should be usable as-is
func (c *Common) SetFatal(fatal bool) {
	c.mx.Lock()
	c.fatal = fatal
	c.mx.Unlock()
}

// Context placeholder implements the State interface
func (c *Common) Context() interface{} {
	return c.context
}

// SetContext placeholder implements the State interface
func (c *Common) SetContext(value interface{}) {
	c.mx.Lock()
	c.context = value
	c.mx.Unlock()
}
