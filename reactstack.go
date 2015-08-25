package flux

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	// ErrFailedBind represent a failure in binding two Reactors
	ErrFailedBind = errors.New("Failed to Bind Reactors")
)

/* Reactor defines an the idea of continous, reactive change which is a revised implementation of FRP principles with a golang view and approach. Reactor are like a reactive queue where each reactor builds off a previous reactor to allow a simple top-down flow of data.
This approach lends itself from very simple streaming operations to complex stream processing systems.
Due to the use of unbuffered channels, Reactor require that the next keep the rule of the channel contract
.i.e a reactor channel must have someone to collect/listen/retrieve the data within it and
ensure a continouse operation else close and end the reactor
*/

// Connector defines the core connecting methods used for binding with a Reactor
type Connector interface {
	//Bind provides a convenient way of binding 2 reactors
	Bind(Reactor) error
	//React generates a reactor based off its caller
	React(ReactiveOpHandler) Reactor
}

// Replier defines reply methods to reply to requests
type Replier interface {
	//reply functions
	Reply(v interface{})
	ReplyClose(v interface{})
	ReplyError(v error)
}

// Sender defines the delivery methods used to deliver data into Reactor process
type Sender interface {
	//delivery functions
	Send(v interface{})
	SendClose(v interface{})
	SendError(v error)
}

// SendBinder defines the combination of the Sender and Binding interfaces
type SendBinder interface {
	Sender
	Connector
}

// Reactor provides an interface definition for the reactor type to allow compatibility by future extenders when composing with other structs.
type Reactor interface {
	Connector
	Sender
	Replier

	Detach()

	//bool functions for ensuring reactors state
	IsHooked() bool
	HasRoot() bool

	//private functions for swapping in reactors
	UseNext(Reactor) error
	UseRoot(Reactor) error
}

// ReactorsView provides a deeper view in the reactor
type ReactorsView interface {
	Reactor
	End()
	Closed() <-chan interface{}
	Signal() <-chan interface{}
	Errors() <-chan error
}

// SignalMuxHandler provides a signal function type
type SignalMuxHandler func(d interface{}) interface{}

// ReactiveOpHandler defines a reactive function operation
type ReactiveOpHandler func(ReactorsView)

// ReactiveStack provides a concrete implementation
type ReactiveStack struct {
	data, closed      chan interface{}
	errs              chan error
	op                ReactiveOpHandler
	root              Reactor
	next              Reactor
	started, finished int64
	ro, rod           sync.Mutex
}

// DistributeSignals takes from one signal and sends it to other reactors
func DistributeSignals(from Reactor, rs ...Sender) (m Reactor) {
	m = from.React(func(view ReactorsView) {
		defer view.End()
	runloop:
		for {
			select {
			case cd := <-view.Closed():
				for n, rsd := range rs {
					func(data interface{}, ind int, ro Sender) {
						GoDefer(fmt.Sprintf("DeliverClose::to(%d)", ind), func() {
							ro.SendClose(data)
						})
					}(cd, n, rsd)
				}
				break runloop
			case dd := <-view.Signal():
				for n, rsd := range rs {

					func(data interface{}, ind int, ro Sender) {
						GoDefer(fmt.Sprintf("DeliverData::to(%d)", ind), func() {
							ro.Send(data)
						})
					}(dd, n, rsd)

				}
			case de := <-view.Errors():
				for n, rsd := range rs {

					func(data interface{}, ind int, ro Sender) {
						GoDefer(fmt.Sprintf("DeliverError::to(%d)", ind), func() {
							ro.Send(data)
						})
					}(de, n, rsd)

				}
			}
		}
	})

	return
}

// MergeReactors takes input from serveral reactors and turn it into one signal (a []interface{}) signal type
func MergeReactors(rs ...SendBinder) Reactor {
	m := ReactIdentity()

	rdo := new(sync.Mutex)
	maxcount := len(rs) - 1

	for _, rsm := range rs {
		func(ro, col SendBinder) {
			ro.React(func(v ReactorsView) {
			mop:
				for {
					select {
					case err := <-v.Errors():
						m.SendError(err)
					case d := <-v.Closed():
						rdo.Lock()
						if maxcount <= 0 {
							m.SendClose(d)
							break mop
						}
						maxcount--
						rdo.Unlock()
					case d := <-v.Signal():
						m.Send(d)
					}
				}
				v.End()
			})
		}(rsm, m)
	}

	return m
}

// LiftReactors takes inputs from each and pipes it to the next reactor
func LiftReactors(rs ...Reactor) (Reactor, error) {
	if len(rs) < 1 {
		return nil, fmt.Errorf("EmptyArgs: Total Count %d", len(rs))
	}

	if len(rs) == 1 {
		return rs[0], nil
	}

	msl := rs[0]
	rs = rs[1:]

	for _, ro := range rs {
		func(rx Reactor) {
			if err := msl.Bind(rx); err == nil {
				msl = rx
			}
		}(ro)
	}

	return msl, nil
}

// DataReactWith wraps the whole data react operation
func DataReactWith(mx Connector, fx SignalMuxHandler) Reactor {
	return mx.React(DataReactProcessor(fx))
}

// DataReact returns a reactor that only sends it in to its out
func DataReact(fx SignalMuxHandler) Reactor {
	return Reactive(DataReactProcessor(fx))
}

// ReactIdentity returns a reactor that only sends it in to its out
func ReactIdentity() Reactor {
	return Reactive(ReactIdentityProcessor())
}

// ReactIdentityProcessor provides the ReactIdentity processing op
func ReactIdentityProcessor() ReactiveOpHandler {
	return func(self ReactorsView) {
		func() {
		iloop:
			for {
				select {
				case d := <-self.Closed():
					self.ReplyClose(d)
					break iloop
				case err := <-self.Errors():
					self.ReplyError(err)
				case data := <-self.Signal():
					self.Reply(data)
				}
			}
			self.End()
		}()
	}
}

// DataReactProcessor provides the internal processing ops for DataReact
func DataReactProcessor(fx SignalMuxHandler) ReactiveOpHandler {
	return func(self ReactorsView) {
		func() {
		iloop:
			for {
				select {
				case d := <-self.Closed():
					self.ReplyClose(d)
					break iloop
				case err := <-self.Errors():
					self.ReplyError(err)
				case data := <-self.Signal():
					self.Reply(fx(data))
				}
			}
			self.End()
		}()
	}
}

// Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
func Reactive(fx ReactiveOpHandler) *ReactiveStack {
	r := &ReactiveStack{
		data:   make(chan interface{}),
		closed: make(chan interface{}),
		errs:   make(chan error),
		op:     fx,
	}

	r.boot()

	return r
}

// Closed returns the error pipe
func (r *ReactiveStack) Closed() <-chan interface{} {
	return r.closed
}

// Errors returns the error pipe
func (r *ReactiveStack) Errors() <-chan error {
	return r.errs
}

// Signal returns the in-put pipe
func (r *ReactiveStack) Signal() <-chan interface{} {
	return r.data
}

// SendError returns the in-put pipe
func (r *ReactiveStack) SendError(d error) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	r.rod.Lock()
	// defer r.rod.Unlock()

	if r.errs == nil {
		return
	}

	r.errs <- d
	r.rod.Unlock()
}

// Send returns the in-put pipe
func (r *ReactiveStack) Send(d interface{}) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	r.rod.Lock()
	// defer r.rod.Unlock()

	if r.data == nil {
		return
	}

	r.data <- d
	r.rod.Unlock()
}

// SendClose returns the in-put pipe
func (r *ReactiveStack) SendClose(d interface{}) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	r.rod.Lock()
	// defer r.rod.Unlock()

	if r.closed == nil {
		return
	}
	r.closed <- d
	r.rod.Unlock()
}

// Reply returns the out-put pipe
func (r *ReactiveStack) Reply(d interface{}) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	if !r.IsHooked() {
		return
	}

	r.next.Send(d)
}

// ReplyClose returns the out-put pipe
func (r *ReactiveStack) ReplyClose(d interface{}) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	if !r.IsHooked() {
		return
	}

	r.next.SendClose(d)
}

// ReplyError returns the out-put pipe
func (r *ReactiveStack) ReplyError(d error) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if d == nil {
		return
	}

	if !r.IsHooked() {
		return
	}

	r.next.SendError(d)
}

// HasRoot returns true/false if its has a chain
func (r *ReactiveStack) HasRoot() bool {
	return r.root != nil
}

// IsHooked returns true/false if its has a chain
func (r *ReactiveStack) IsHooked() bool {
	r.ro.Lock()
	state := (r.next != nil)
	r.ro.Unlock()
	return state
}

// Bind connects a reactor to the next available reactor in the chain that has no binding,you can only bind if the provided reactor has no binding (root) and if the target reactor has no next. A bool value is returned to indicate success or failure
func (r *ReactiveStack) Bind(fx Reactor) error {
	if err := r.UseNext(fx); err != nil {
		return r.next.Bind(fx)
	}

	fx.UseRoot(r)
	return nil
}

// React creates a reactivestack from this current one
func (r *ReactiveStack) React(fx ReactiveOpHandler) Reactor {

	if r.next != nil {
		return r.next.React(fx)
	}

	nx := Reactive(fx)
	nx.root = r

	r.next = nx

	return r.next
}

// End signals to the next stack its closing
func (r *ReactiveStack) End() {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	atomic.StoreInt64(&r.finished, 1)

	if r.root != nil {
		r.root.Detach()
		r.root = nil
	}
}

// UseRoot allows setting the root Reactor if there is non set
func (r *ReactiveStack) UseRoot(fx Reactor) error {
	if r.root != nil {
		return ErrFailedBind
	}
	r.root = fx
	return ErrFailedBind
}

// UseNext allows setting the next Reactor if there is non set
func (r *ReactiveStack) UseNext(fx Reactor) error {
	if r.next != nil {
		return ErrFailedBind
	}

	r.next = fx
	return nil
}

// Detach nullifies the next link of this Reactor
func (r *ReactiveStack) Detach() {
	r.ro.Lock()
	r.next = nil
	r.ro.Unlock()
}

// ForceRun forces the immediate start of the reactor
func (r *ReactiveStack) boot() {
	//bootup this reactor
	if r.started > 0 {
		return
	}

	atomic.StoreInt64(&r.started, 1)
	GoDefer("StartReact", func() {
		r.op(r)
	})
}

// ChannelStream provides a simple struct for exposing outputs from Reactor to outside
type ChannelStream struct {
	Close  chan interface{}
	Data   chan interface{}
	Errors chan error
}

// NewChannelStream returns a new channel stream instance
func NewChannelStream() *ChannelStream {
	return &ChannelStream{
		Close:  make(chan interface{}),
		Data:   make(chan interface{}),
		Errors: make(chan error),
	}
}

// ChannelReact builds a ChannelStream directly with a Reactor
func ChannelReact(c *ChannelStream) Reactor {
	return Reactive(ChannelProcessor(c))
}

// ChannelReactWith provides a factory to create a Reactor to a channel
func ChannelReactWith(mx Connector, c *ChannelStream) Reactor {
	return mx.React(ChannelProcessor(c))
}

// ChannelProcessor provides the ReactIdentity processing op
func ChannelProcessor(c *ChannelStream) ReactiveOpHandler {
	return func(self ReactorsView) {
		func() {
		iloop:
			for {
				select {
				case d := <-self.Closed():
					GoDefer("ChannelClose", func() {
						c.Close <- d
					})
					self.ReplyClose(d)
					break iloop
				case err := <-self.Errors():
					GoDefer("ChannelError", func() {
						c.Errors <- err
					})
					self.ReplyError(err)
				case data := <-self.Signal():
					GoDefer("ChannelData", func() {
						c.Data <- data
					})
					self.Reply(data)
				}
			}
			self.End()
		}()
	}
}
