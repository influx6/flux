package flux

import "sync"

// FlatReactor provides a pure functional reactor
type FlatReactor struct {
	op SignalMuxHandler
	// root,next Reactor
	branches, enders, roots *mapReact
	csignal                 chan bool
	wo                      sync.Mutex
	wg                      sync.WaitGroup
	closed                  bool
}

// FlatIdentity returns flatreactor that resends its inputs as outputs with no changes
func FlatIdentity() *FlatReactor {
	return FlatReactive(func(r Reactor, err error, d interface{}) {
		if err != nil {
			r.ReplyError(err)
			return
		}
		r.Reply(d)
	})
}

// FlatReactive returns a new functional reactor
func FlatReactive(op SignalMuxHandler) *FlatReactor {
	fr := FlatReactor{
		op:       op,
		branches: NewMapReact(),
		enders:   NewMapReact(),
		roots:    NewMapReact(),
		csignal:  make(chan bool),
	}

	return &fr
}

// FlatStack returns a flat reactor
func FlatStack() Reactor {
	return ReactStack(FlatIdentity())
}

// UseRoot Adds this reactor as a root of the called reactor
func (f *FlatReactor) UseRoot(rx Reactor) {
	f.roots.Add(rx)
}

// Detach removes the given reactor from its connections
func (f *FlatReactor) Detach(rm Reactor) {
	f.enders.Disable(rm)
	f.branches.Disable(rm)
}

// Bind connects two reactors
func (f *FlatReactor) Bind(rx Reactor, cl bool) {
	f.branches.Add(rx)
	rx.UseRoot(f)

	if cl {
		f.enders.Add(rx)
	}
}

// React builds a new reactor from this one
func (f *FlatReactor) React(op SignalMuxHandler, cl bool) Reactor {
	nx := FlatReactive(op)
	nx.UseRoot(f)

	f.branches.Add(nx)

	if cl {
		f.enders.Add(nx)
	}

	return nx
}

// Manage basically we do nothing here,since its functionally flat
func (f *FlatReactor) Manage() {
	return
}

// CloseNotify provides a channel for notifying a close event
func (f *FlatReactor) CloseNotify() <-chan bool {
	return f.csignal
}

// Close closes the reactor and removes all connections
func (f *FlatReactor) Close() error {
	if f.closed {
		return nil
	}

	f.wg.Wait()
	f.closed = true

	f.branches.Close()

	f.roots.Do(func(rm SenderDetachCloser) {
		rm.Detach(f)
	})

	f.roots.Close()
	f.enders.Close()

	close(f.csignal)
	return nil
}

// Send applies a message value to the handler
func (f *FlatReactor) Send(b interface{}) {
	f.wg.Add(1)
	defer f.wg.Done()
	f.op(f, nil, b)
}

// SendError applies a error value to the handler
func (f *FlatReactor) SendError(err error) {
	f.wg.Add(1)
	defer f.wg.Done()
	f.op(f, err, nil)
}

//Reply allows the reply of an data message
func (f *FlatReactor) Reply(v interface{}) {
	f.branches.Deliver(nil, v)
}

//ReplyError allows the reply of an error message
func (f *FlatReactor) ReplyError(err error) {
	f.branches.Deliver(err, nil)
}
