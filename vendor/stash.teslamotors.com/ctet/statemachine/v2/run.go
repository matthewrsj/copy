package statemachine

import (
	"sync"
)

// RunFrom runs the state machine starting at s. It runs infinitely until a
// state defines itself as Last()
func RunFrom(s State) {
	for {
		RunOne(s)

		// this state identifies as last, end the statemachine
		if s.Last() {
			break
		}

		// grab the next state and do it all again
		s = s.Next()
	}
}

// RunOne runs the actions of a single state
func RunOne(s State) {
	var wg sync.WaitGroup
	// all actions to be performed by the current state
	actions := s.Actions()
	wg.Add(len(actions))

	for _, f := range actions {
		// invoke each action in a goroutine so we can use
		// the waitgroup to wait for them all to finish
		go func(f func()) {
			// f() is blocking, so wg.Done() will be called when f()
			// is done executing
			defer wg.Done()
			f()
		}(f) // pass in f on each iteration to avoid races
	}

	// when this clears all the actions for this state have completed
	wg.Wait()
}