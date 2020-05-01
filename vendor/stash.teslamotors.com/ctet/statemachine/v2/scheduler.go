package statemachine

import (
	"fmt"
	"sync"
)

// Scheduler provides a registry of key names to states to run. Each registered state
// runs its machine sequentially before accepting another job. To run a single state
// machine concurrently with itself use the Pool type.
type Scheduler struct {
	registry sync.Map
	jobs     chan Job
	done     bool
	mx       *sync.Mutex
	wg       *sync.WaitGroup
}

// Job is the work to be completed by the State at Name
// Work can be whatever the user wants. To utilize the Work value the user must provide a custom runner function
// when registering the State.
type Job struct {
	Name string
	Work interface{}
}

// NewScheduler returns a pointer to a new Scheduler object
func NewScheduler() *Scheduler {
	return &Scheduler{
		jobs: make(chan Job, 1),
		mx:   &sync.Mutex{},
		wg:   &sync.WaitGroup{},
	}
}

// syncState is the internal structure for handling a state and its individual job queue
type syncState struct {
	state State
	q     chan Job
}

// SchedulerDefaultRunner is the default runner when a State receives a new job. This runner sets the j.Work as the
// State's context and then calls RunFrom(s)
func SchedulerDefaultRunner(s State, j Job) {
	s.SetContext(j.Work)
	RunFrom(s)
}

// Register registers a State to the scheduler. If runner is nil the SchedulerDefaultRunner is used
func (s *Scheduler) Register(key string, state State, runner func(State, Job)) {
	ss := syncState{
		state: state,
		q:     make(chan Job, 1),
	}

	s.registry.Store(key, ss)

	// only replace the runner if it is non-nil
	if runner == nil {
		runner = SchedulerDefaultRunner
	}

	s.wg.Add(1)
	go func(ss syncState) {
		defer s.wg.Done()
		for job := range ss.q {
			runner(state, job)
		}
	}(ss)
}

// Schedule schedules new jobs to be run by Run()
func (s *Scheduler) Schedule(job Job) error {
	v, ok := s.registry.Load(job.Name)
	if !ok {
		return fmt.Errorf("job %s not in registry", job.Name)
	}

	ss, ok := v.(syncState)
	if !ok {
		return fmt.Errorf("job %s type %T not permitted", job, v)
	}

	ss.q <- job
	return nil
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

// Done signals that no new jobs are coming
func (s *Scheduler) Done() {
	if s.done {
		return
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	s.registry.Range(func(key, value interface{}) bool {
		v, ok := value.(syncState)
		if !ok {
			// still want to try to close other channels
			return true
		}

		close(v.q)
		return true
	})

	s.done = true
	close(s.jobs)
}

// End is a blocking call that calls Done() followed by Wait() on the scheduler
func (s *Scheduler) End() {
	s.Done()
	s.Wait()
}
