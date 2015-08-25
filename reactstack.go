package flux

import (
	"sync"
	"sync/atomic"
)

type (

	//Signal denotes a value received by a reactivestack
	Signal interface{}

	//ReactiveOp defines a reactive function operation
	ReactiveOp func(ReactorsView)

	//Reactors provides an interface for a stack implementation using channels
	Reactors interface {
		React(ReactiveOp) Reactors
		detach()
		IsHooked() bool
		Send(d Signal)
		SendClose(d Signal)
		SendError(d error)
		Reply(d Signal)
		ReplyClose(d Signal)
		ReplyError(d error)
		Closed() <-chan Signal
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
		cleaner           *sync.Once
	}
)

// ReactIdentity returns a reactor that only sends it in to its out
func ReactIdentity() Reactors {
	return Reactive(func(self ReactorsView) {
		func() {
			defer self.End()
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
		}()
	}, nil)
}

//Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
func Reactive(fx ReactiveOp, root Reactors) *ReactiveStack {
	r := &ReactiveStack{
		data:    make(chan Signal),
		closed:  make(chan Signal),
		errs:    make(chan error),
		op:      fx,
		root:    root,
		cleaner: new(sync.Once),
	}

	r.boot()

	return r
}

func (r *ReactiveStack) detach() {
	r.next = nil
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
	if r.finished > 0 {
		return
	}

	if d == nil {
		return
	}

	r.errs <- d
}

//Send returns the in-put pipe
func (r *ReactiveStack) Send(d Signal) {
	if r.finished > 0 {
		return
	}

	if d == nil {
		return
	}

	r.data <- d
}

//SendClose returns the in-put pipe
func (r *ReactiveStack) SendClose(d Signal) {
	if r.finished > 0 {
		return
	}

	if r == nil {
		return
	}

	if r.next == nil {
		return
	}

	r.closed <- d
}

//Reply returns the out-put pipe
func (r *ReactiveStack) Reply(d Signal) {
	if r.finished > 0 {
		return
	}

	if d == nil {
		return
	}

	if r.next == nil {
		return
	}

	r.next.Send(d)
}

//ReplyClose returns the out-put pipe
func (r *ReactiveStack) ReplyClose(d Signal) {
	if r.finished > 0 {
		return
	}

	if d == nil {
		return
	}

	if r.next == nil {
		return
	}

	r.next.SendClose(d)
}

//ReplyError returns the out-put pipe
func (r *ReactiveStack) ReplyError(d error) {
	if r.finished > 0 {
		return
	}

	if d == nil {
		return
	}

	if r.next == nil {
		return
	}

	r.next.SendError(d)
}

//View returns this stack as a reactiveView
func (r *ReactiveStack) View() ReactorsView {
	return r
}

// IsHooked returns true/false if its has a chain
func (r *ReactiveStack) IsHooked() bool {
	return r.next != nil
}

//React creates a reactivestack from this current one
func (r *ReactiveStack) React(fx ReactiveOp) Reactors {

	if r.next != nil {
		return r.next.React(fx)
	}

	r.next = Reactive(fx, r)

	return r.next
}

//End signals to the next stack its closing
func (r *ReactiveStack) End() {

	if r.finished > 0 {
		return
	}

	atomic.StoreInt64(&r.finished, 1)
	GoDefer("CloseReact", func() {
		if r.root != nil {
			r.root.detach()
		}
		close(r.data)
		close(r.errs)
		close(r.closed)
	})
}
