package flux

import (
	"io"
	"log"
	"sync/atomic"
)

type (

	//HalfStackable defines a stackable function type that can return a value
	HalfStackable func(interface{})

	//Stackable defines a stackable function type that can return a value
	Stackable func(Stacks, interface{}) interface{}

	//MultiStackable defines a stackable function type that can return a value but accepts multi-arguments
	MultiStackable func(Stacks, ...interface{}) interface{}

	//NStacks provides a variation over the stack implementation for multi-argument functions
	// NStacks interface {
	// 	//Identity applies the same arguments to all connected stacks
	// 	Identity(...interface{}) interface{}
	//
	// 	//isolate applies to this stack only and does not propagate the value to other stacks
	// 	Isolate(...interface{}) interface{}
	//
	// 	//Call runs the value, returning the return value of just this particular stacks return value
	// 	Call(...interface{}) interface{}
	//
	// 	//Apply applies the value, returning the values of either this stack or the child stacks if any exists
	// 	Apply(...interface{}) interface{}
	//
	// 	//Lift lifts this value up the root stack for use with the root .Apply() function(allows you to call a root .Apply() method from anylevel of the stack)
	// 	Lift(...interface{}) interface{}
	//
	// 	//LiftApply calls the stack from bottom up and takes the result from a lower stack to apply to its parent stack and returns the root return value
	// 	LiftApply(...interface{}) interface{}
	//
	// 	//Levitate calls the stack from bottom up feeding the root with the returned value from the child but returns the child returned value
	// 	Levitate(...interface{}) interface{}
	// }

	//OneStack defines emission function taking only one argument
	OneStack interface {
		//Isolate applies to this stack only and does not propagate the value to other stacks
		Isolate(interface{}) interface{}

		//Call runs the value, returning the return value of just this particular stacks return value
		Call(interface{}) interface{}

		//Apply applies the value, returning the values of either this stack or the child stacks if any exists
		Apply(interface{}) interface{}

		//Lift lifts this value up the root stack for use with the root .Apply() function(allows you to call a root .Apply() method from anylevel of the stack)
		Lift(interface{}) interface{}

		//LiftApply calls the stack from bottom up and takes the result from a lower stack to apply to its parent stack and returns the root return value
		LiftApply(interface{}) interface{}

		//Levitate calls the stack from bottom up feeding the root with the returned value from the child but returns the child returned value
		Levitate(interface{}) interface{}

		//Identity applies the same arguments to all connected stacks
		Identity(interface{}) interface{}
	}

	//Stacks provides the finite definition of function stacks
	Stacks interface {
		// NStacks
		OneStack
		Stack(Stackable, bool) Stacks
		Listen(HalfStackable) Stacks
		Unstack()
		// MaxArgs() int
		//Channel returns a receive only channel for notification instead of using the callback approach with Listen or stack
		Channel() <-chan interface{}

		//Close closes the stacks
		Close() error

		//getWrap returns the stack wrapper
		getWrap() *StackWrap

		//LockClose clocks the provided stack with this,ensuring to close it when this stack gets closed
		LockClose(Stacks)
	}

	//StackWrap provides a single-layer two function binder
	StackWrap struct {
		Fn      Stackable
		next    *StackWrap
		owner   Stacks
		closefx func()
	}

	//Stack is the concrete definition of Stacks
	Stack struct {
		wrap   *StackWrap
		root   Stacks
		active int64
		args   int
	}

	//StackStreamers provides a streaming function for stacks member rules
	StackStreamers interface {
		Stacks
		StreamWriter(io.Writer) Stacks
		Write([]byte) (int, error)
		Read([]byte) (int, error)
		Closed() Stacks
	}

	//StackStream is the concrete implementation for stacks
	StackStream struct {
		Stacks
		closeNotifier Stacks
	}

	//Reports provides a abstraction of report
	Reports struct {
		Tag  string
		Meta map[string]interface{}
	}

	//StackReport provides a internal bank of reports
	StackReport struct {
		stacks chan *Reports
		ended  int64
	}
)

//WrapStackable takes a function and turns it to a Stackable
func WrapStackable(fx func(interface{}) interface{}) Stackable {
	return func(_ Stacks, x interface{}) interface{} {
		return fx(x)
	}
}

//WrapNArg wraps a multiarg function into a single arg function
func WrapNArg(max int, fx MultiStackable) Stackable {
	return func(s Stacks, x interface{}) interface{} {
		mset, ok := x.([]interface{})

		if ok {
			return fx(s, mset[:max]...)
		}

		var oset []interface{}
		oset = append(oset, x)
		return fx(s, oset...)
	}
}

//WrapNStackable wraps a multiarg function to a mult-arg stackable function
func WrapNStackable(max int, fx func(...interface{}) interface{}) Stackable {
	return WrapNArg(max, func(_ Stacks, b ...interface{}) interface{} {
		return fx(b...)
	})
}

//StackableIdentity provides an identity function for stacks
func StackableIdentity(_ Stacks, b interface{}) interface{} {
	return b
}

//ReportCard returns a new StackReport instance, generally ensure its a good max so you can get reports without over-filling the stack,if the stack is full then old reports would be discard
func ReportCard(max int) *StackReport {
	return &StackReport{
		stacks: make(chan *Reports, max),
	}
}

//Checkout returns a Stack which gets filled with the current and possible future reports
func (r *StackReport) Checkout() StackStreamers {
	cs := NewIdentityStream()
	GoDefer("CheckoutReport", func() {
		for {

			state := atomic.LoadInt64(&r.ended)

			if state > 0 {
				break
			}

			select {
			case v, ok := <-r.stacks:
				if !ok {
					return
				}
				cs.Call(v)
			}
		}
	})
	return cs
}

//Report adds a new report to the reportstack
func (r *StackReport) Report(tag string, meta map[string]interface{}) {
	state := atomic.LoadInt64(&r.ended)

	if state > 0 {
		return
	}

	curcap := cap(r.stacks)
	curlen := len(r.stacks)

	if curlen >= curcap {
		GoDefer("DiscardTopReport", func() {
			<-r.stacks
		})
	}

	r.stacks <- &Reports{tag, meta}
}

//Close ends the report card operations
func (r *StackReport) Close() {
	state := atomic.LoadInt64(&r.ended)
	if state > 0 {
		return
	}

	atomic.StoreInt64(&r.ended, 1)
	close(r.stacks)
}

//NewIdentityStream  returns new StackStream
func NewIdentityStream() StackStreamers {
	return NewStackStream(StackableIdentity)
}

//StreamReader streams from a reader,hence all values are replaced by the data in the io.Reader
func StreamReader(w io.Reader) (s StackStreamers) {
	s = NewStackStream(StackableIdentity)
	go func() {
		_, _ = io.Copy(s, w)
	}()
	return
}

//Read reads into a byte but its empty
func (s *StackStream) Read(bu []byte) (int, error) {
	return 0, io.ErrNoProgress
}

//Write writes into the stream
func (s *StackStream) Write(bu []byte) (int, error) {
	s.Call(bu)
	return len(bu), nil
}

//StreamWriter streams to a writer
func (s *StackStream) StreamWriter(w io.Writer) Stacks {
	return s.Stack(func(_ Stacks, data interface{}) interface{} {
		buff, ok := data.([]byte)

		if !ok {

			str, ok := data.(string)

			if ok {
				_, _ = w.Write([]byte(str))
			}

		}
		_, _ = w.Write(buff)
		return data
	}, true)
}

//Closed returns a stack that is emitted when closed
func (s *StackStream) Closed() Stacks {
	return s.closeNotifier
}

//Close ends this stacks connections
func (s *StackStream) Close() error {
	defer s.closeNotifier.Close()
	s.closeNotifier.Call(true)
	s.Stacks.Close()
	return nil
}

//NewStackStream returns a new stackstream
func NewStackStream(mx Stackable) StackStreamers {
	return &StackStream{
		Stacks:        NewStack(mx),
		closeNotifier: NewStack(StackableIdentity),
	}
}

//Close destroys the stackwrap
func (s *StackWrap) Close() {
	if s.next != nil {
		if s.next.owner != nil {
			s.next.owner.Close()
		}
	}
	s.Fn = nil
	s.owner = nil
	if s.closefx != nil {
		defer func() { s.closefx = nil }()
		s.closefx()
	}
}

//Unwrap unwraps a Stackwrap for release
func (s *StackWrap) Unwrap() *StackWrap {
	nx := s.next
	s.next = nil
	return nx
}

//Rewrap wraps a new Stackwrap aw next
func (s *StackWrap) Rewrap(sc *StackWrap) *StackWrap {
	nx := s.Unwrap()
	s.next = sc
	return nx
}

//NewStackWrap returns a new Stackwrap
func NewStackWrap(fn Stackable, own Stacks, fx func()) *StackWrap {
	return &StackWrap{Fn: fn, owner: own, next: nil, closefx: fx}
}

//NewStack returns a new stack
func NewStack(fn Stackable) (s *Stack) {
	s = &Stack{
		active: 1,
		args:   1,
	}

	s.wrap = NewStackWrap(fn, s, nil)
	return
}

// //NArgStack returns a new stack
// func NArgStack(max int, fn MultiStackable) (s *Stack) {
// 	if max <= 0 {
// 		max = 1
// 	}
// 	s = &Stack{
// 		active: 1,
// 		args:   max,
// 	}
//
// 	s.wrap = NewStackWrap(WrapNArg(max, fn), s, nil)
// 	return
// }

//IdentityStack returns a stack that returns its values
func IdentityStack() *Stack {
	return NewStack(StackableIdentity)
}

// //MaxArgs returns the maximum argument this stack takes
// func (s *Stack) MaxArgs() int {
// 	return s.args
// }

//Isolate applies to this stack only and does not propagate the value to other stacks
func (s *Stack) Isolate(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(s, b)
	}
	return res
}

//Identity runs the value returning the return value of just this particular stacks return value
func (s *Stack) Identity(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(s, b)
		if s.wrap != nil {
			if s.wrap.next != nil {
				if s.wrap.next.owner != nil {
					_ = s.wrap.next.owner.Identity(b)
				}
			}
		}
	}
	return res
}

//Call runs the value returning the return value of just this particular stacks return value
func (s *Stack) Call(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(s, b)
		if s.wrap != nil {
			if s.wrap.next != nil {
				if s.wrap.next.owner != nil {
					_ = s.wrap.next.owner.Call(res)
				}
			}
		}
	}
	return res
}

//Apply sends a data into the stack and returns this stack
//returned value
func (s *Stack) Apply(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(s, b)
		if s.wrap != nil {
			if s.wrap.next != nil {
				if s.wrap.next.owner != nil {
					return s.wrap.next.owner.Apply(res)
				}
			}
		}
	}
	return res
}

//Lift allows the passing of a value to the root of a tree before executing an emit downwards
func (s *Stack) Lift(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	if s.root != nil {
		return s.root.Lift(b)
	}

	return s.Apply(b)
}

//Levitate takes a value,mutates it then passes up to the root, moving from bottom-up and returns the roots returned value
func (s *Stack) Levitate(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	res := s.wrap.Fn(s, b)

	if s.root != nil {
		s.root.Levitate(res)
		// return s.root.LiftApply(res)
	}

	return res
}

//LiftApply takes a value,mutates it then pass that to the root. moving from bottom-to-top
func (s *Stack) LiftApply(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	res := s.wrap.Fn(s, b)

	if s.root != nil {
		return s.root.LiftApply(res)
	}

	return res
}

//Close destroys the stack and any other chain it has
func (s *Stack) Close() error {
	if s.wrap == nil {
		return nil
	}

	defer func() { s.wrap = nil }()
	s.Unstack()
	s.wrap.Close()
	return nil
}

//Unstack disables and nullifies this stack
func (s *Stack) Unstack() {
	if s.wrap == nil || s.root == nil {
		return
	}

	root := s.root
	nxt := s.wrap.next

	atomic.StoreInt64(&s.active, 0)
	root.getWrap().Rewrap(nxt)
	s.root = nil
}

//getWrap returns the stacks wrapper
func (s *Stack) getWrap() *StackWrap {
	return s.wrap
}

//Listen builds a new stack from this previous stack
func (s *Stack) Listen(fn HalfStackable) Stacks {
	return s.Stack(func(_ Stacks, b interface{}) interface{} {
		fn(b)
		return b
	}, true)
}

//Channel returns a unbuffered channel for notification
func (s *Stack) Channel() <-chan interface{} {
	mc := make(chan interface{})
	s.Listen(func(data interface{}) {
		GoDefer("Stack-Channel-Deliver", func() {
			mc <- data
		})
	})
	return mc
}

//LockClose locks the stacks to the close of this
func (s *Stack) LockClose(m Stacks) {
	cx := s.getWrap().closefx
	s.getWrap().closefx = func() {
		m.Close()
		if cx != nil {
			cx()
		}
	}
}

//Stack builds a new stack from this previous stack, the connectClose defines whether you want the new Stack closed when its parent is closed,if false then we dont close it when the root closes
func (s *Stack) Stack(fn Stackable, connectClose bool) Stacks {
	if s.wrap == nil {
		return nil
	}

	if s.wrap.next != nil {
		if s.wrap.next.owner != nil {
			return s.wrap.next.owner.Stack(fn, connectClose)
		}
	}

	m := NewStack(fn)
	m.root = s

	s.wrap.next = m.wrap

	if connectClose {
		s.LockClose(m)
	}

	return m
}

//LogStack takes a stack and logs all input coming from it
func LogStack(s Stacks) Stacks {
	return s.Stack(func(_ Stacks, data interface{}) interface{} {
		log.Printf("LogStack: %+s", data)
		return data
	}, true)
}

//LogHeader takes a stack and logs all input from it
func LogHeader(s Stacks, header string) Stacks {
	return s.Stack(func(_ Stacks, data interface{}) interface{} {
		log.Printf(header, data)
		return data
	}, true)
}

//LogStackWith logs all input using a custom logger
func LogStackWith(s Stacks, l *log.Logger) Stacks {
	return s.Stack(func(_ Stacks, data interface{}) interface{} {
		l.Printf("LogStack: %+s", data)
		return data
	}, true)
}

//BoolOnly ensures only boolean value pass through
func BoolOnly(s Stacks) Stacks {
	sm := NewIdentityStream()

	s.Listen(func(data interface{}) {
		bl, ok := data.(bool)
		if ok {
			sm.Apply(bl)
		}
	})

	s.LockClose(sm)

	return sm
}

//CombineStacks returns a new stack
func CombineStacks(mo ...Stacks) Stacks {
	co := IdentityStack()

	for _, ms := range mo {
		ms.Listen(func(b interface{}) {
			co.Call(b)
		})
	}

	return co
}

//FeedStacks returns a new stack
func FeedStacks(mi, mo Stacks, kill bool) Stacks {
	return mi.Stack(func(_ Stacks, b interface{}) interface{} {
		return mo.Apply(b)
	}, kill)
}

//FeedAllStacks returns a new stack
func FeedAllStacks(kill bool, m ...Stacks) Stacks {
	var rs Stacks

	for _, m := range m {
		if rs == nil {
			rs = m
			continue
		}
		rs = FeedStacks(rs, m, kill)
	}

	return rs
}
