package flux

import "sync/atomic"

type (

	//Signal denotes a value received by a reactivestack
	Signal interface{}

	//ReactiveOp defines a reactive function operation
	ReactiveOp func(ReactiveStacks)

	//ReactiveStacks provides an interface for a stack implementation using channels
	ReactiveStacks interface {
		In() chan Signal
		Out() chan Signal
		Error() <-chan error
		Closed() <-chan struct{}
		Feed() ReactiveStacks
		HasChild() bool
		// Child() ReactiveStacks
		React(ReactiveOp) ReactiveStacks
		End()
		detach()
	}

	//ReactiveStack provides a concrete implementation
	ReactiveStack struct {
		in, out           chan Signal
		closed            chan struct{}
		errs              chan error
		flow              bool
		op                ReactiveOp
		root              ReactiveStacks
		next              ReactiveStacks
		started, finished int64
	}
)

//ReactReceive returns a react operator
func ReactReceive() ReactiveOp {
	return func(self ReactiveStacks) {
		func() {
		iloop:
			for {
				select {
				case <-self.Feed().Closed():
					self.End()
					break iloop
				case <-self.Closed():
					break iloop
				case data := <-self.Feed().Out():
					self.Out() <- data
				case data := <-self.In():
					self.Out() <- data
				}
			}
		}()
	}
}

//ReactIdentity returns a reactor that only sends it in to its out
func ReactIdentity() ReactiveStacks {
	return Reactive(func(self ReactiveStacks) {
		func() {
		iloop:
			for {
				select {
				case <-self.Closed():
					break iloop
				case data := <-self.In():
					if self.HasChild() {
						go func() { self.Out() <- data }()
					} else {
						data = nil
					}
				}
			}
		}()
	}, nil)
}

//Reactive returns a ReactiveStacks,the process is not started immediately if no root exists,to force it,call .ForceRun()
func Reactive(fx ReactiveOp, root ReactiveStacks) *ReactiveStack {
	r := &ReactiveStack{
		in:     make(chan Signal),
		out:    make(chan Signal),
		closed: make(chan struct{}),
		errs:   make(chan error),
		op:     fx,
		root:   root,
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

	GoDefer("StartReact", func() {
		atomic.StoreInt64(&r.started, 1)
		r.op(r)
	})
}

//In returns the in-put pipe
func (r *ReactiveStack) In() chan Signal {
	return r.in
}

//Out returns the out-put pipe
func (r *ReactiveStack) Out() chan Signal {
	return r.out
}

//Closed returns the error pipe
func (r *ReactiveStack) Closed() <-chan struct{} {
	return r.closed
}

//Error returns the error pipe
func (r *ReactiveStack) Error() <-chan error {
	return r.errs
}

//Feed returns the parent reativestack
func (r *ReactiveStack) Feed() ReactiveStacks {
	return r.root
}

//HasChild returns true/false if its has a chain
func (r *ReactiveStack) HasChild() bool {
	return r.next != nil
}

//Parent returns the parent reativestack
// func (r *ReactiveStack) Parent() ReactiveStacks {
// 	return r.root
// }
//Child returns the next reativestack
// func (r *ReactiveStack) Child() ReactiveStacks {
// 	return r.next
// }

//React creates a reactivestack from this current one
func (r *ReactiveStack) React(fx ReactiveOp) ReactiveStacks {

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

	GoDefer("CloseReact", func() {
		r.root.detach()
		close(r.closed)
		atomic.StoreInt64(&r.finished, 1)
	})
}
