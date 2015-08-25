package flux

import (
	"fmt"
	"sync"
	"sync/atomic"
)

//Reactors defines an the idea of continous, reactive change which is a revised implementation of FRP principles with a golang view and approach. Reactors are like a reactive queue where each reactor builds off a previous reactor to allow a simple top-down flow of data. This approach lends itself from very simple streaming operations to complex stream processing systems. Due to the use of unbuffered channels, Reactors require that the next keep the rule of the channel contract .i.e a reactor channel must have someone to collect/listen/retrieve the data within it i.e ensure a continouse operation else close and end the reactor

type (

	//Reactors provides an interface definition for the reactor type to allow compatibility by future extenders when composing with other structs.
	Reactors interface {
		detach()

		//Bind provides a convenient way of binding 2 reactors
		Bind(Reactors) bool

		//React generates a reactor based off its caller
		React(ReactiveOpHandler) Reactors

		//bool functions for ensuring reactors state
		IsHooked() bool
		HasRoot() bool

		//delivery functions
		Send(d interface{})
		SendClose(d interface{})
		SendError(d error)

		//reply functions
		Reply(d interface{})
		ReplyClose(d interface{})
		ReplyError(d error)

		//private functions for swapping in reactors
		useNext(Reactors) bool
		useRoot(Reactors) bool
	}

	//ReactorsView provides a deeper view in the reactor
	ReactorsView interface {
		Reactors
		End()
		Closed() <-chan interface{}
		Signal() <-chan interface{}
		Errors() <-chan error
	}

	//SignalMuxHandler provides a signal function type
	SignalMuxHandler func(d interface{}) interface{}

	//ReactiveOpHandler defines a reactive function operation
	ReactiveOpHandler func(ReactorsView)

	//ReactiveStack provides a concrete implementation
	ReactiveStack struct {
		data, closed      chan interface{}
		errs              chan error
		op                ReactiveOpHandler
		root              Reactors
		next              Reactors
		started, finished int64
		ro, rod           sync.Mutex
	}
)

//DistributeSignals takes from one signal and sends it to other reactors
func DistributeSignals(from Reactors, rs ...Reactors) (m Reactors) {
	m = from.React(func(view ReactorsView) {
		defer view.End()

	runloop:
		for {
			select {
			case cd := <-view.Closed():
				for n, rsd := range rs {
					func(data interface{}, ind int, ro Reactors) {
						GoDefer(fmt.Sprintf("DeliverClose::to(%d)", ind), func() {
							ro.SendClose(data)
						})
					}(cd, n, rsd)
				}
				break runloop
			case dd := <-view.Signal():
				for n, rsd := range rs {

					func(data interface{}, ind int, ro Reactors) {
						GoDefer(fmt.Sprintf("DeliverData::to(%d)", ind), func() {
							ro.Send(data)
						})
					}(dd, n, rsd)

				}
			case de := <-view.Errors():
				for n, rsd := range rs {

					func(data interface{}, ind int, ro Reactors) {
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

//MergeReactors takes input from serveral reactors and turn it into one signal (a []interface{}) signal type
func MergeReactors(rs ...Reactors) Reactors {
	m := ReactIdentity()

	rdo := new(sync.Mutex)
	maxcount := len(rs) - 1

	for _, rsm := range rs {
		func(ro, col Reactors) {
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

//LiftReactors takes inputs from each and pipes it to the next reactor
func LiftReactors(rs ...Reactors) (Reactors, error) {
	if len(rs) < 1 {
		return nil, fmt.Errorf("EmptyArgs: Total Count %d", len(rs))
	}

	if len(rs) == 1 {
		return rs[0], nil
	}

	msl := rs[0]
	rs = rs[1:]

	for _, ro := range rs {
		func(rx Reactors) {
			msl.Bind(rx)
			msl = rx
		}(ro)
	}

	return msl, nil
}

//DataReactWith wraps the whole data react operation
func DataReactWith(mx Reactors, fx SignalMuxHandler) Reactors {
	return mx.React(DataReactProcessor(fx))
}

//DataReact returns a reactor that only sends it in to its out
func DataReact(fx SignalMuxHandler) Reactors {
	return Reactive(DataReactProcessor(fx))
}

// ReactIdentity returns a reactor that only sends it in to its out
func ReactIdentity() Reactors {
	return Reactive(ReactIdentityProcessor())
}

//ReactIdentityProcessor provides the ReactIdentity processing op
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

//DataReactProcessor provides the internal processing ops for DataReact
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

//Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
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

//Closed returns the error pipe
func (r *ReactiveStack) Closed() <-chan interface{} {
	return r.closed
}

//Errors returns the error pipe
func (r *ReactiveStack) Errors() <-chan error {
	return r.errs
}

// Signal returns the in-put pipe
func (r *ReactiveStack) Signal() <-chan interface{} {
	return r.data
}

//SendError returns the in-put pipe
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

//Send returns the in-put pipe
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

//SendClose returns the in-put pipe
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

//Reply returns the out-put pipe
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

//ReplyClose returns the out-put pipe
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

//ReplyError returns the out-put pipe
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

//Bind connects a reactor to the next available reactor in the chain that has no binding,you can only bind if the provided reactor has no binding (root) and if the target reactor has no next. A bool value is returned to indicate success or failure
func (r *ReactiveStack) Bind(fx Reactors) bool {
	if !r.useNext(fx) {
		return r.next.Bind(fx)
	}

	fx.useRoot(r)
	return true
}

//React creates a reactivestack from this current one
func (r *ReactiveStack) React(fx ReactiveOpHandler) Reactors {

	if r.next != nil {
		return r.next.React(fx)
	}

	nx := Reactive(fx)
	nx.root = r

	r.next = nx

	return r.next
}

//End signals to the next stack its closing
func (r *ReactiveStack) End() {
	state := atomic.LoadInt64(&r.finished)

	if state > 0 {
		return
	}

	atomic.StoreInt64(&r.finished, 1)

	if r.root != nil {
		r.root.detach()
	}
}

func (r *ReactiveStack) useRoot(fx Reactors) bool {
	if r.root != nil {
		return false
	}
	r.root = fx
	return true
}

func (r *ReactiveStack) useNext(fx Reactors) bool {
	if r.next != nil {
		return false
	}

	r.next = fx
	return true
}

func (r *ReactiveStack) detach() {
	r.ro.Lock()
	r.next = nil
	r.ro.Unlock()
}

//ForceRun forces the immediate start of the reactor
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

type (
	//ChannelStream provides a simple struct for exposing outputs from Reactors to outside
	ChannelStream struct {
		Close  chan interface{}
		Data   chan interface{}
		Errors chan error
	}
)

//NewChannelStream returns a new channel stream instance
func NewChannelStream() *ChannelStream {
	return &ChannelStream{
		Close:  make(chan interface{}),
		Data:   make(chan interface{}),
		Errors: make(chan error),
	}
}

//ChannelReact builds a ChannelStream directly with a Reactor
func ChannelReact(c *ChannelStream) Reactors {
	return Reactive(ChannelProcessor(c))
}

//ChannelReactWith provides a factory to create a Reactor to a channel
func ChannelReactWith(mx Reactors, c *ChannelStream) Reactors {
	return mx.React(ChannelProcessor(c))
}

//ChannelProcessor provides the ReactIdentity processing op
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
