package statemachine

import (
	"sync"
)

// Pool is a worker pool to run states. Pool follows the common idioms for worker
// thread pools, but runs state machines. By default Pool runs each state machine using
// RunFrom when a job is received. The user can override this default runner if they wish.
type Pool struct {
	generator  func() State
	numWorkers int
	jobs       chan interface{}
	runner     Runner
	mx         *sync.Mutex
	wg         *sync.WaitGroup
	done       bool
}

// Runner function generates a new worker to handle the job coming in.
// It is important that defer p.WGDone() is called within the function definition
type Runner func(*Pool)

// NewPool creates a new *Pool with which to run asynchronous states
func NewPool(generator func() State, numWorkers int) *Pool {
	p := &Pool{
		generator:  generator,
		numWorkers: numWorkers,
		jobs:       make(chan interface{}, 1), // buffer of one so first write doesn't block
		mx:         &sync.Mutex{},
		wg:         &sync.WaitGroup{},
	}
	p.runner = RunFromRunner
	return p
}

// Reset resets all the state of the Pool. This can be called after Done() is called to continue using the same pool.
func (p *Pool) Reset() {
	*p = *NewPool(p.generator, p.numWorkers)
}

// SetGenerator sets the generator function that generates the states to handle jobs.
// The generator function can construct a new state instance (if context of the specific job matters)
// or reuse the same state for each job (if context of the specific job does not matter).
func (p *Pool) SetGenerator(generator func() State) {
	p.mx.Lock()
	defer p.mx.Unlock()
	p.generator = generator
}

// Worker returns the worker state by calling the generator function.
func (p *Pool) Worker() State {
	return p.generator()
}

// NumWorkers returns the number of workers this Pool is configured for
func (p *Pool) NumWorkers() int {
	return p.numWorkers
}

// SetNumWorkers updates the number of workers this Pool is configured for to num
func (p *Pool) SetNumWorkers(num int) {
	p.mx.Lock()
	defer p.mx.Unlock()
	p.numWorkers = num
}

// SetRunner returns the runner function for the Pool.
// The Runner is the function that starts the worker statemachine. Default Runner function calls RunFrom(p.Worker())
// when a new job is received. This function can be used to change that logic in order to get values from the Jobs
// channel instead of just using the job as a sentinel.
func (p *Pool) SetRunner(f Runner) {
	p.mx.Lock()
	defer p.mx.Unlock()
	p.runner = f
}

// Run spawns p.NumWorkers() Runners in goroutines. Run does not block.
// if Done() has been called this does nothing
func (p *Pool) Run() {
	if p.done {
		return
	}

	p.wg.Add(p.numWorkers)
	for i := 0; i < p.numWorkers; i++ {
		go p.runner(p)
	}
}

// Wait waits for all workers to be done
func (p *Pool) Wait() {
	p.wg.Wait()
}

// Jobs returns the jobs channel
func (p *Pool) Jobs() chan interface{} {
	return p.jobs
}

// AddJob adds a sentinel job to the job channel
// if Done() has been called this does nothing
func (p *Pool) AddJob() {
	p.AddJobValue(struct{}{})
}

// AddJobValue adds a job containing v to the jobs channel
// if Done() has been called this does nothing
func (p *Pool) AddJobValue(v interface{}) {
	if p.done {
		return
	}

	p.jobs <- v
}

// Done closes the jobs channel
func (p *Pool) Done() {
	if p.done {
		return
	}

	p.done = true
	close(p.jobs)
}

// End is a blocking call that calls Done() followed by Wait() on the pool
func (p *Pool) End() {
	p.Done()
	p.Wait()
}

// WGDone calls Done() on the pools internal waitgroup. This function must be used inside any custom runner
// provided by the user to Pool.SetRunner()
func (p *Pool) WGDone() {
	p.wg.Done()
}

// RunFromRunner is a Runner function that calls RunFrom(p.Worker()) when a job is received on p.Jobs()
// ignores the contents of the job
func RunFromRunner(p *Pool) {
	defer p.WGDone()
	for range p.Jobs() {
		RunFrom(p.Worker())
	}
}

// RunOneRunner is a Runner function that calls RunOne(p.Worker()) when a job is received on p.Jobs()
// ignores the contents of the job
func RunOneRunner(p *Pool) {
	defer p.WGDone()
	for range p.Jobs() {
		RunOne(p.Worker())
	}
}
