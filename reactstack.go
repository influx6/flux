package flux

import (
	"fmt"
	"log"
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
		View() ReactorsView
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
	})
}

//Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
func Reactive(fx ReactiveOp) *ReactiveStack {
	r := &ReactiveStack{
		data:    make(chan Signal),
		closed:  make(chan Signal),
		errs:    make(chan error),
		op:      fx,
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

	nx := Reactive(fx)
	nx.root = r

	r.next = nx

	return r.next
}

//End signals to the next stack its closing
func (r *ReactiveStack) End() {
	if r.finished > 0 {
		return
	}

	atomic.StoreInt64(&r.finished, 1)
	if r.root != nil {
		r.root.detach()
	}
	close(r.data)
	close(r.errs)
	close(r.closed)
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
							log.Printf("sending error", ind)
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
func MergeReactors(rs ...Reactors) (m Reactors) {
	return nil
}
