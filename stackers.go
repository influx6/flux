package flux

import (
	"io"
	"log"
	"sync/atomic"
)

type (

	//Stackable defines a stackable function type that can return a value
	Stackable func(interface{}, Stacks) interface{}

	//Stacks provides the finite definition of function stacks
	Stacks interface {
		Stack(Stackable) Stacks
		Unstack()
		Emit(interface{}) interface{}
		Mux(interface{}) interface{}
		getWrap() *StackWrap
	}

	//StackWrap provides a single-layer two function binder
	StackWrap struct {
		Fn    Stackable
		next  *StackWrap
		owner Stacks
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
		Close() error
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
	})
}

//Closed returns a stack that is emitted when closed
func (s *StackStream) Closed() Stacks {
	return s.closeNotifier
}

//Close ends this stacks connections
func (s *StackStream) Close() error {
	s.closeNotifier.Emit(true)
	s.Unstack()
	return nil
}

//NewStackStream returns a new stackstream
func NewStackStream(mx Stackable) StackStreamers {
	return &StackStream{
		Stacks:        NewStack(mx, nil),
		closeNotifier: NewStack(StackableIdentity, nil),
	}
}

// //Die sets this wrapper as garbage
// func (s *StackWrap) Free() {
// 	s.Fn,s.owner = nil,nil
// }

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
func NewStackWrap(fn Stackable, own Stacks) *StackWrap {
	return &StackWrap{Fn: fn, owner: own, next: nil}
}

//NewStack returns a new stack
func NewStack(fn Stackable, root Stacks) (s *Stack) {
	s = &Stack{
		active: 1,
		root:   root,
	}

	s.wrap = NewStackWrap(fn, s)
	return
}

//Emit sends a data into the stack and returns this stack
//returned value
func (s *Stack) Emit(b interface{}) interface{} {
	if b == nil {
		return nil
	}
	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(b, s)
		if s.wrap.next != nil {
			if s.wrap.next.owner != nil {
				_ = s.wrap.next.owner.Emit(b)
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
	var res interface{}
	state := atomic.LoadInt64(&s.active)
	if state > 0 {
		res = s.wrap.Fn(b, s)
		if s.wrap.next != nil {
			if s.wrap.next.owner != nil {
				if res != nil {
					s.wrap.next.owner.Emit(res)
				}
			}
		}
	}
	return res
}

//Unstack disables and nullifies this stack
func (s *Stack) Unstack() {
	if s.root == nil {
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

//Stack builds a new stack from this previous stack
func (s *Stack) Stack(fn Stackable) Stacks {
	if s.wrap.next != nil {
		return s.wrap.next.owner.Stack(fn)
	}
	m := NewStack(fn, s)
	s.wrap.next = m.wrap
	return m
}

//LogStack takes a stack and logs all input coming from it
func LogStack(s Stacks) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		log.Printf("LogStack: %+s", data)
		return data
	})
}

//LogHeader takes a stack and logs all input from it
func LogHeader(s Stacks, header string) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		log.Printf(header, data)
		return data
	})
}

//LogStackWith logs all input using a custom logger
func LogStackWith(s Stacks, l *log.Logger) Stacks {
	return s.Stack(func(data interface{}, _ Stacks) interface{} {
		l.Printf("LogStack: %+s", data)
		return data
	})
}
