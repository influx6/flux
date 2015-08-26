package flux

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

/* Reactors define an the idea of continous, reactive change which is a revised implementation of FRP principles with a golang view and approach. Reactor are like a reactive queue where each reactor builds off a previous reactor to allow a simple top-down flow of data.
This approach lends itself from very simple streaming operations to complex stream processing systems.
Due to the use of unbuffered channels, Reactor require that the next keep the rule of the channel contract
.i.e a reactor channel must have someone to collect/listen/retrieve the data within it and
ensure a continouse operation else close and end the reactor
*/

// ErrFailedBind represent a failure in binding two Reactors
var ErrFailedBind = errors.New("Failed to Bind Reactors")

// ErrReactorClosed returned when reactor is closed
var ErrReactorClosed = errors.New("Reactor is Closed")

// SignalMuxHandler provides a signal function type:
/*
  It takes three arguments:
		- reactor:(Reactor) the reactor itself for reply processing
		- failure:(error) the current error being returned when a data is nil
		- data:(interface{}) the current data being returned,nil when theres an error
*/
type SignalMuxHandler func(reactor Reactor, failure error, signal interface{})

// Reactor provides an interface definition for the reactor type to allow compatibility by future extenders when composing with other structs.
type Reactor interface {
	io.Closer
	Connector
	Sender
	Replier
	Detacher

	UseRoot(Reactor)
	Manage()
}

// ReactiveStack provides a concrete implementation
type ReactiveStack struct {
	ps                    *PressureStream
	op                    SignalMuxHandler
	root                  Reactor
	branch, enders, roots *mapReact
	wg                    sync.WaitGroup
	ro                    sync.Mutex
	bit                   int64
}

//ReactIdentityProcessor provides the processor for a ReactIdentity
func ReactIdentityProcessor() SignalMuxHandler {
	return func(r Reactor, err error, data interface{}) {
		if err != nil {
			r.ReplyError(err)
			return
		}
		r.Reply(data)
	}
}

//ReactIdentity returns a reactor that passes on its request to its listeners if any without modification
func ReactIdentity() Reactor {
	return Reactive(ReactIdentityProcessor())
}

// Reactive returns a ReactiveStacks
func Reactive(fx SignalMuxHandler) *ReactiveStack {
	r := BuildReactive(fx)
	go r.Manage()
	return r
}

// BuildReactive returns a Reactor without calling the Manager processor,this is to allow a more control managment of the operation of the Reactor e.g pass the process up to a Work Pool
func BuildReactive(fx SignalMuxHandler) *ReactiveStack {
	data := make(chan interface{})
	errs := make(chan interface{})

	r := ReactiveStack{
		ps:     BuildPressureStream(data, errs),
		branch: NewMapReact(),
		enders: NewMapReact(),
		roots:  NewMapReact(),
		op:     fx,
	}

	return &r
}

// Close ends signaling operation to the next stack its closing
func (r *ReactiveStack) Close() error {
	if atomic.LoadInt64(&r.bit) > 0 {
		return ErrReactorClosed
	}

	r.roots.Do(func(rm SenderDetachCloser) {
		rm.Detach(r)
	})

	r.wg.Wait()
	atomic.StoreInt64(&r.bit, 1)

	r.ps.Close()
	r.roots.Clean()
	r.branch.Clean()
	return nil
}

//UseRoot adds a new root as part of the reactor roots
func (r *ReactiveStack) UseRoot(rx Reactor) {
	r.roots.Add(rx)
}

// Manage forces the immediate start of the reactor
func (r *ReactiveStack) Manage() {
	defer func() {
		r.enders.Close()
		r.roots.Clean()
		r.branch.Clean()
	}()

	for {
		select {
		case err, ok := <-r.ps.Errors:
			if !ok {
				return
			}
			r.wg.Done()
			// go r.op(r, err.(error), nil)
			r.op(r, err.(error), nil)
		case signal, ok := <-r.ps.Signals:
			if !ok {
				return
			}
			r.wg.Done()
			// go r.op(r, nil, signal)
			r.op(r, nil, signal)
		}
	}
}

//Detacher details the detach interface used by the Reactor
type Detacher interface {
	Detach(Reactor)
}

// Detach nullifies the next link of this Reactor
func (r *ReactiveStack) Detach(rm Reactor) {
	r.enders.Disable(rm)
	r.branch.Disable(rm)
}

// Sender defines the delivery methods used to deliver data into Reactor process
type Sender interface {
	Send(v interface{})
	SendError(v error)
}

// SendError returns the in-put pipe
func (r *ReactiveStack) SendError(e error) {
	r.wg.Add(1)
	r.ps.SendError(e)
}

// Send returns the in-put pipe
func (r *ReactiveStack) Send(d interface{}) {
	r.wg.Add(1)
	r.ps.SendSignal(d)
}

//DistributeSignals provide a function that takes a React and other multiple Reactor and distribute the data from the first reactor to others
func DistributeSignals(rs Reactor, ms ...Sender) Reactor {
	return rs.React(func(r Reactor, err error, data interface{}) {
		for _, mi := range ms {
			go func(rec Sender) {
				if err != nil {
					rec.SendError(err)
					return
				}
				rec.Send(data)
			}(mi)
		}
	}, true)
}

// Replier defines reply methods to reply to requests
type Replier interface {
	Reply(v interface{})
	ReplyError(v error)
}

// Reply returns the out-put pipe
func (r *ReactiveStack) Reply(d interface{}) {
	r.branch.Deliver(nil, d)
}

// ReplyError returns the out-put pipe
func (r *ReactiveStack) ReplyError(err error) {
	r.branch.Deliver(err, nil)
}

// Connector defines the core connecting methods used for binding with a Reactor
type Connector interface {
	//Bind provides a convenient way of binding 2 reactors
	Bind(r Reactor, closeAlong bool)
	//React generates a reactor based off its caller
	React(s SignalMuxHandler, closeAlong bool) Reactor
	BindControl(r Reactor, fx func())
}

// Bind connects a reactor to this reactor as an alternative to a connection by the React() approach
func (r *ReactiveStack) Bind(rx Reactor, closeAlong bool) {

	r.branch.Add(rx)
	rx.UseRoot(r)

	if closeAlong {
		r.enders.Add(rx)
	}
}

// BindControl provides a means of adding an extra control layer for binding,it runs a go-routine that waits till the root is closed to run the supplied function
func (r *ReactiveStack) BindControl(rx Reactor, fx func()) {
	r.branch.Add(rx)
	// r.enders.Add(rx)
	rx.UseRoot(r)

	go func(ro sync.WaitGroup) {
		ro.Wait()
		fx()
	}(r.wg)
}

// React creates a reactivestack from this current one with a boolean value
func (r *ReactiveStack) React(fx SignalMuxHandler, closeAlong bool) Reactor {
	nx := Reactive(fx)
	nx.UseRoot(r)

	r.branch.Add(nx)
	if closeAlong {
		r.enders.Add(nx)
	}
	return nx
}

// SendBinder defines the combination of the Sender and Binding interfaces
type SendBinder interface {
	Sender
	Connector
}

// MergeReactors merges data from a set of Senders into a new reactor stream
func MergeReactors(rs ...SendBinder) Reactor {
	ms := ReactIdentity()
	for _, si := range rs {
		func(so SendBinder) {
			var cu int
			size := len(rs)
			so.BindControl(ms, func() {
				if cu == size {
					ms.Close()
					return
				}
				cu++
			})
		}(si)
	}
	return ms
}

//ReplierCloser provides an interface that combines Replier and Closer interfaces
type ReplierCloser interface {
	Replier
	io.Closer
}

//SenderCloser provides an interface that combines Sender and Closer interfaces
type SenderCloser interface {
	Sender
	io.Closer
}

//SenderDetachCloser provides an interface that combines Sender and Closer interfaces
type SenderDetachCloser interface {
	Sender
	Detacher
	io.Closer
}

// MapReact provides a nice way of adding multiple reacts into a reactor reply list.
type mapReact struct {
	ro sync.RWMutex
	ma map[SenderDetachCloser]bool
}

// NewMapReact returns a new MapReact store
func NewMapReact() *mapReact {
	ma := mapReact{ma: make(map[SenderDetachCloser]bool)}
	return &ma
}

func (m *mapReact) Clean() {
	m.ro.Lock()
	m.ma = nil
	m.ro.Unlock()
}

func (m *mapReact) Deliver(err error, data interface{}) {
	m.ro.RLock()
	if m.ma != nil {
		for ms, ok := range m.ma {
			if !ok {
				continue
			}

			if err != nil {
				ms.SendError(err)
				continue
			}

			ms.Send(data)
		}
	}
	m.ro.RUnlock()
}

func (m *mapReact) Add(r SenderDetachCloser) {
	m.ro.Lock()
	if m.ma != nil {
		if !m.ma[r] {
			m.ma[r] = true
		}
	}
	m.ro.Unlock()
}

func (m *mapReact) Disable(r SenderDetachCloser) {
	m.ro.Lock()
	if m.ma != nil {
		if _, ok := m.ma[r]; ok {
			m.ma[r] = false
		}
	}
	m.ro.Unlock()
}

func (m *mapReact) Length() int {
	var l int
	m.ro.RLock()
	if m.ma != nil {
		l = len(m.ma)
	}
	m.ro.RUnlock()
	return l
}

func (m *mapReact) Do(fx func(SenderDetachCloser)) {
	m.ro.RLock()
	if m.ma != nil {
		for ms := range m.ma {
			fx(ms)
		}
	}
	m.ro.RUnlock()
}

func (m *mapReact) DisableAll() {
	m.ro.Lock()
	if m.ma != nil {
		for ms := range m.ma {
			m.ma[ms] = false
		}
	}
	m.ro.Unlock()
}

func (m *mapReact) Close() {
	m.ro.RLock()
	if m.ma != nil {
		for ms, ok := range m.ma {
			if !ok {
				continue
			}
			ms.Close()
		}
	}
	m.ro.RUnlock()
}

// ChannelStream provides a simple struct for exposing outputs from Reactor to outside
type ChannelStream struct {
	Data   chan interface{}
	Errors chan error
}

// NewChannelStream returns a new channel stream instance
func NewChannelStream() *ChannelStream {
	return &ChannelStream{
		Data:   make(chan interface{}),
		Errors: make(chan error),
	}
}
