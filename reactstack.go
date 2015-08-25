package flux

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type (

	//Signal denotes a value received by a reactivestack
	Signal interface{}

	//SignalMux provides a signal function type
	SignalMux func(d Signal) Signal

	//ReactiveOp defines a reactive function operation
	ReactiveOp func(ReactorsView)

	//Reactors provides an interface for a stack implementation using channels
	Reactors interface {
		Bind(Reactors) bool
		React(ReactiveOp) Reactors
		detach()
		Destroy()
		View() ReactorsView
		IsHooked() bool
		HasRoot() bool
		Send(d Signal)
		SendClose(d Signal)
		SendError(d error)
		Reply(d Signal)
		ReplyClose(d Signal)
		ReplyError(d error)
		Closed() <-chan Signal
		useNext(Reactors) bool
		useRoot(Reactors) bool
	}

	//ReactorsView provides a deeper view in the reactor
	ReactorsView interface {
		Reactors
		End()
		Signal() <-chan Signal
		Errors() <-chan error
	}

	//ReactiveStack provides a concrete implementation
	ReactiveStack struct {
		data, closed      chan Signal
		errs              chan error
		op                ReactiveOp
		root              Reactors
		next              Reactors
		started, finished int64
		ro                *sync.Mutex
	}
)

//ReactIdentityProcessor provides the ReactIdentity processing op
func ReactIdentityProcessor() ReactiveOp {
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
			self.Destroy()
		}()
	}
}

// ReactIdentity returns a reactor that only sends it in to its out
func ReactIdentity() Reactors {
	return Reactive(ReactIdentityProcessor())
}

//DataReactProcessor provides the internal processing ops for DataReact
func DataReactProcessor(fx SignalMux) ReactiveOp {
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
			self.Destroy()
		}()
	}
}

//DataReact returns a reactor that only sends it in to its out
func DataReact(fx SignalMux) Reactors {
	return Reactive(DataReactProcessor(fx))
}

//Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
func Reactive(fx ReactiveOp) *ReactiveStack {
	r := &ReactiveStack{
		data:   make(chan Signal),
		closed: make(chan Signal),
		errs:   make(chan error),
		op:     fx,
		ro:     new(sync.Mutex),
	}

	r.boot()

	return r
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

//Closed returns the error pipe
func (r *ReactiveStack) Closed() <-chan Signal {
	return r.closed
}

//Errors returns the error pipe
func (r *ReactiveStack) Errors() <-chan error {
	return r.errs
}

// Signal returns the in-put pipe
func (r *ReactiveStack) Signal() <-chan Signal {
	return r.data
}

//SendError returns the in-put pipe
func (r *ReactiveStack) SendError(d error) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if r.errs == nil {
		return
	}

	if d == nil {
		return
	}

	r.errs <- d
}

//Send returns the in-put pipe
func (r *ReactiveStack) Send(d Signal) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if r.data == nil {
		return
	}

	if d == nil {
		return
	}

	r.data <- d
}

//SendClose returns the in-put pipe
func (r *ReactiveStack) SendClose(d Signal) {
	state := atomic.LoadInt64(&r.finished)
	if state > 0 {
		return
	}

	if r.closed == nil {
		return
	}

	if d == nil {
		return
	}

	r.closed <- d
}

//Reply returns the out-put pipe
func (r *ReactiveStack) Reply(d Signal) {
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
func (r *ReactiveStack) ReplyClose(d Signal) {
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

//View returns this stack as a reactiveView
func (r *ReactiveStack) View() ReactorsView {
	return r
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
func (r *ReactiveStack) React(fx ReactiveOp) Reactors {

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

//Destroy closes the channels after the call to End()
func (r *ReactiveStack) Destroy() {
	state := atomic.LoadInt64(&r.finished)
	if state <= 0 {
		return
	}

	close(r.data)
	close(r.errs)
	close(r.closed)

	r.data = nil
	r.errs = nil
	r.closed = nil
}

//DistributeSignals takes from one signal and sends it to other reactors
func DistributeSignals(from Reactors, rs ...Reactors) (m Reactors) {
	m = from.React(func(view ReactorsView) {
		defer view.End()

	runloop:
		for {
			select {
			case cd := <-view.Closed():
				for n, rsd := range rs {
					func(data Signal, ind int, ro Reactors) {
						GoDefer(fmt.Sprintf("DeliverClose::to(%d)", ind), func() {
							ro.SendClose(data)
						})
					}(cd, n, rsd)
				}
				break runloop
			case dd := <-view.Signal():
				for n, rsd := range rs {

					func(data Signal, ind int, ro Reactors) {
						GoDefer(fmt.Sprintf("DeliverData::to(%d)", ind), func() {
							ro.Send(data)
						})
					}(dd, n, rsd)

				}
			case de := <-view.Errors():
				for n, rsd := range rs {

					func(data Signal, ind int, ro Reactors) {
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
				v.Destroy()

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
