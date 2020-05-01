// Package statemachine provides an interface to run generic States in a standard way.
// The package provides the State interface as well as runners to run the state machine.
//
// Additional orchestration features such as Pool and Scheduler allow users to create
// scalable dynamic systems that run asynchronous state machines based on triggers.
//
// Example usage of these features can be found under examples/ in the source code.
package statemachine

// State is a single state in a state machine. It provides setters and getters for internal state
// and is expected to be run by calling state.Actions(), invoking each action function returned,
// then transitioning to the next state via state.Next(). See the provided runners RunFrom and RunOne.
type State interface {
	// Name returns the name of this state
	Name() string
	// SetName sets the state name to the passed parameter
	SetName(string)

	// Actions performs whatever actions this state defines. The action functions
	// returned are run concurrently and must call wg.Done() before exiting.
	Actions() []func()

	// Next returns the next state to be run after the current state
	Next() State

	// Last identifies whether this state is the last state in the sequence
	// state runners can use this to either identify the last state in a single
	// run or to identify when to stop infinite state runs
	Last() bool
	// SetLast sets the internal state to the passed parameter
	SetLast(bool)

	// Fatal identifies whether a fatal error was encountered during the
	// state's Action function and can be used to control whether the
	// state machine should prematurely end
	Fatal() bool
	// SetFatal sets the internal fatal state to the passed parameter
	SetFatal(bool)

	// Context returns the data set by SetContext
	Context() interface{}
	// SetContext stores the data in the state for use by the runner or user if desired
	SetContext(interface{})
}

// NewState returns a new state with name as its identifier
func NewState(s State, name string) State {
	s.SetName(name)
	return s
}
