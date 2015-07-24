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
	Stackable func(interface{}, Stacks) interface{}

	//Stacks provides the finite definition of function stacks
	Stacks interface {
		Stack(Stackable, bool) Stacks
		Listen(HalfStackable) Stacks
		Unstack()
		Emit(interface{}) interface{}
		Mux(interface{}) interface{}
		getWrap() *StackWrap
		Close() error
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
)

var (
	//StackableIdentity provides an identity function for stacks
	StackableIdentity = func(b interface{}, _ Stacks) interface{} {
		return b
	}
)

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
	s.Emit(bu)
	return len(bu), nil
}

//StreamWriter streams to a writer
func (s *StackStream) StreamWriter(w io.Writer) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
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
	s.closeNotifier.Emit(true)
	s.Stacks.Close()
	return nil
}

//NewStackStream returns a new stackstream
func NewStackStream(mx Stackable) StackStreamers {
	return &StackStream{
		Stacks:        NewStack(mx, nil, true),
		closeNotifier: NewStack(StackableIdentity, nil, true),
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
func NewStack(fn Stackable, root Stacks, closeOnRoot bool) (s *Stack) {
	s = &Stack{
		active: 1,
		root:   root,
	}

	if closeOnRoot && root != nil {
		cx := root.getWrap().closefx
		root.getWrap().closefx = func() {
			s.Close()
			if cx != nil {
				cx()
			}
		}
	}

	s.wrap = NewStackWrap(fn, s, nil)
	return
}

//Emit sends a data into the stack and returns this stack
//returned value
func (s *Stack) Emit(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(b, s)
		if s.wrap != nil {
			if s.wrap.next != nil {
				if s.wrap.next.owner != nil {
					_ = s.wrap.next.owner.Emit(b)
				}
			}
		}
	}
	return res
}

//Mux lets you mutate the next passed data to the next
//stack,that is this stack return value is the next stacks input
func (s *Stack) Mux(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	if s.wrap == nil {
		return nil
	}

	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(b, s)
		if s.wrap != nil {
			if s.wrap.next != nil {
				if s.wrap.next.owner != nil {
					if res != nil {
						s.wrap.next.owner.Emit(res)
					}
				}
			}
		}
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
	return s.Stack(func(b interface{}, _ Stacks) interface{} {
		fn(b)
		return b
	}, true)
}

//Stack builds a new stack from this previous stack, the connectClose defines whether you want the new Stack closed when its parent is closed,if false then we dont close it when the root closes
func (s *Stack) Stack(fn Stackable, connectClose bool) Stacks {
	if s.wrap == nil {
		return nil
	}

	if s.wrap.next != nil {
		return s.wrap.next.owner.Stack(fn, connectClose)
	}
	m := NewStack(fn, s, connectClose)
	s.wrap.next = m.wrap
	return m
}

//LogStack takes a stack and logs all input coming from it
func LogStack(s Stacks) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		log.Printf("LogStack: %+s", data)
		return data
	}, true)
}

//LogHeader takes a stack and logs all input from it
func LogHeader(s Stacks, header string) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		log.Printf(header, data)
		return data
	}, true)
}

//LogStackWith logs all input using a custom logger
func LogStackWith(s Stacks, l *log.Logger) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		l.Printf("LogStack: %+s", data)
		return data
	}, true)
}
