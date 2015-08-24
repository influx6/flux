package flux

import (
	"sync"
	"sync/atomic"
)

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
		mux               *ReactiveMultiplier
		started, finished int64
	}

	//ReactiveProxy creates a proxy over a root reactor
	reactiveProxy struct {
		ReactiveStacks
		index int
		pout  chan Signal
		mul   *ReactiveMultiplier
	}

	//ReactiveMultiplier creates a multi-distributor for reactors
	ReactiveMultiplier struct {
		root    ReactiveStacks
		multis  []*reactiveProxy
		rw      *sync.RWMutex
		openids []int
	}
)

//NewReactiveProxy returns a reactive proxy
func NewReactiveProxy(ro *ReactiveMultiplier, ind int) *reactiveProxy {
	return &reactiveProxy{
		ReactiveStacks: ro.root,
		pout:           make(chan Signal),
		mul:            ro,
		index:          ind,
	}
}

//NewMultiplier returns a new ReactiveMultiplier
func NewMultiplier(ro ReactiveStacks) (rr *ReactiveMultiplier) {

	rfx := func(self ReactiveStacks) {
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
					rr.handleMessage(data)
				case data := <-self.In():
					rr.handleMessage(data)
				}
			}
		}()
	}

	rod := &ReactiveStack{
		in:     make(chan Signal),
		out:    make(chan Signal),
		closed: make(chan struct{}),
		errs:   make(chan error),
		op:     rfx,
		root:   ro,
	}

	rr = &ReactiveMultiplier{
		root: rod,
		rw:   new(sync.RWMutex),
	}

	rod.boot()

	return
}

func (r *ReactiveMultiplier) handleMessage(d Signal) {
	if len(r.multis) <= 0 {
		return
	}

	r.rw.RLock()
	defer r.rw.RUnlock()

	for _, fxm := range r.multis {
		if fxm == nil {
			continue
		}
		func(rm *reactiveProxy, signal Signal) {
			GoDefer("deliver-to-proxy", func() {
				rm.pout <- signal
			})
		}(fxm, d)
	}
}

//Remove unsets the proxy at the id and frees for reuse
func (r *ReactiveMultiplier) remove(d int) {
	r.rw.RLock()
	r.multis[d] = nil
	r.rw.RUnlock()
	r.openids = append(r.openids, d)
}

//React creates a reactivestack from this current one
func (r *ReactiveMultiplier) React(fx ReactiveOp) ReactiveStacks {
	var opx *reactiveProxy

	sz := len(r.multis)
	osz := len(r.openids)

	if osz > 0 {
		ind := r.openids[osz-1]
		r.openids = r.openids[:osz]

		opx = NewReactiveProxy(r, ind)
		r.multis[ind] = opx
	} else {
		r.openids = r.openids[:0]
		opx = NewReactiveProxy(r, sz)
		r.multis = append(r.multis, opx)
	}

	// Reactive(fx, r)
	return Reactive(fx, opx)
}

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
					go func() { self.Out() <- data }()
				case data := <-self.In():
					go func() { self.Out() <- data }()
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
				case data, ok := <-self.In():
					if !ok {
						return
					}
					go func() {
						self.Out() <- data
					}()
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

	r.mux = NewMultiplier(r)

	r.boot()

	return r
}

func (r *reactiveProxy) detach() {
	r.mul.remove(r.index)
}

func (r *ReactiveStack) detach() {
	// r.next = nil
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

//Out returns the out-put pipe of the proxy
func (r *reactiveProxy) Out() chan Signal {
	return r.pout
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

//React creates a reactivestack from this current one
func (r *ReactiveStack) React(fx ReactiveOp) ReactiveStacks {
	return r.mux.React(fx)
}

//End signals to the next stack its closing
func (r *ReactiveStack) End() {

	if r.finished > 0 {
		return
	}

	atomic.StoreInt64(&r.finished, 1)
	GoDefer("CloseReact", func() {
		close(r.closed)
		close(r.in)
		close(r.out)
		r.in = nil
		r.out = nil

		if r.root != nil {
			r.root.detach()
		}
	})
}
