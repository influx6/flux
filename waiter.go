package flux

import (
	"sync"
	"sync/atomic"
	"time"
)

//WaitInterface defines the flux.Wait interface method definitions
type WaitInterface interface {
	Add()
	Done()
	Count() int
	Flush()
	Then() ActionInterface
}

//SwitchInterface defines a flux.Switch interface method definition
type SwitchInterface interface {
	Switch()
	IsOn() bool
	WhenOn() ActionInterface
	WhenOff() ActionInterface
}

type baseWait struct {
	action ActionInterface
}

//Then returns an ActionInterface which gets fullfilled when this wait
//counter reaches zero
func (w *baseWait) Then() ActionInterface {
	return w.action.Wrap()
}

func newBaseWait() *baseWait {
	return &baseWait{NewAction()}
}

//TimeWait defines a time lock waiter
type TimeWait struct {
	*baseWait
	closer chan struct{}
	hits   int64
	ms     time.Duration
	doonce *sync.Once
}

//NewTimeWait returns a new timer wait locker
func NewTimeWait(max int, duration time.Duration) *TimeWait {

	if max <= 0 {
		max = 1
	}

	tm := &TimeWait{
		newBaseWait(),
		make(chan struct{}),
		int64(max),
		duration,
		new(sync.Once),
	}

	go tm.handle()

	return tm
}

//handle effects the necessary time process for checking and reducing the
//time checker for each duration of time,till the Waiter is done
func (w *TimeWait) handle() {
	closed := false

	go func() {
		<-w.closer
		closed = true
	}()

	for {
		time.Sleep(w.ms)

		if closed {
			break
		}

		w.Done()
	}
}

//Flush drops the lock count and forces immediate unlocking of the wait
func (w *TimeWait) Flush() {
	w.doonce.Do(func() {
		close(w.closer)
		w.action.Fullfill(0)
		w.hits = int64(0)
	})
}

//Count returns the total left count to completed before unlock
func (w *TimeWait) Count() int {
	return int(w.hits)
}

//Add increments the lock state to the lock counter unless its already unlocked
func (w *TimeWait) Add() {
	if w.Count() < 0 {
		return
	}

	atomic.AddInt64(&w.hits, 1)
}

//Done decrements the totalcount of this waitlocker by 1 until its below zero
//and fullfills with the 0 value
func (w *TimeWait) Done() {
	hits := atomic.LoadInt64(&w.hits)

	if hits < 0 {
		return
	}

	newhit := atomic.AddInt64(&w.hits, -1)

	if int(newhit) < 0 {
		w.Flush()
	}
}

//Wait implements the WiatInterface for creating a wait lock which
//waits until the lock lockcount is finished then executes a action
//can only be used once, that is ,once the wait counter is -1,you cant add
//to it anymore
type Wait struct {
	*baseWait
	totalCount int64
}

//NewWait returns a new Wait instance for the WaitInterface
func NewWait() WaitInterface {
	return &Wait{newBaseWait(), int64(0)}
}

//Flush drops the lock count and forces immediate unlocking of the wait
func (w *Wait) Flush() {
	curr := int(atomic.LoadInt64(&w.totalCount))
	for curr >= 0 {
		w.Done()
		curr--
	}
}

//Count returns the total left count to completed before unlock
func (w *Wait) Count() int {
	return int(atomic.LoadInt64(&w.totalCount))
}

//Add increments the lock state to the lock counter unless its already unlocked
func (w *Wait) Add() {
	curr := atomic.LoadInt64(&w.totalCount)

	if curr < 0 {
		return
	}

	atomic.AddInt64(&w.totalCount, 1)
}

//Done decrements the totalcount of this waitlocker by 1 until its below zero
//and fullfills with the 0 value
func (w *Wait) Done() {
	curr := atomic.LoadInt64(&w.totalCount)

	if curr < 0 {
		return
	}

	nc := atomic.AddInt64(&w.totalCount, -1)

	if int(nc) < 0 {
		w.action.Fullfill(0)
	}
}
