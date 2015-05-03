package flux

import "github.com/influx6/sequence"

//SocketInterface defines member function rules
type SocketInterface interface {
	Emit(interface{})
	Stream(chan interface{})
	addListenerIndex(func(interface{})) int
	removeListenerIndex(int)
	Subscribe(func(interface{}, *Sub)) *Sub
	Size() int
	PoolSize() int
}

//Sub provides a nice clean subscriber connection for socket
type Sub struct {
	socket SocketInterface
	pin    int
	fnx    func(interface{}, *Sub)
}

//Close disconnects the subscription
func (s *Sub) Close() {
	if s.socket == nil {
		return
	}
	s.socket.removeListenerIndex(s.pin)
	s.socket = nil
	s.pin = 0
}

//NewSub returns a new subscriber
func NewSub(sock SocketInterface, fn func(interface{}, *Sub)) *Sub {
	sd := &Sub{sock, -1, fn}
	sd.pin = sock.addListenerIndex(func(v interface{}) {
		sd.fnx(v, sd)
	})
	return sd
}

//Socket is the base structure for all data flow communication
type Socket struct {
	channel    chan interface{}
	listeners  sequence.RootSequencable
	bufferSize int
}

//PoolSize returns the size of data in the channel
func (s *Socket) PoolSize() int {
	return s.bufferSize
}

//Subscribe returns a subscriber
func (s *Socket) Subscribe(fx func(interface{}, *Sub)) *Sub {
	return NewSub(s, fx)
}

//Size returns the size of data in the channel
func (s *Socket) Size() int {
	return len(s.channel)
}

//Emit adds a new data into the channel
func (s *Socket) Emit(b interface{}) {
	s.channel <- b
}

//RemoveListenerIndex adds a function into the socket queue
func (s *Socket) removeListenerIndex(ind int) {
	s.listeners.Delete(ind)
}

//AddListenerIndex adds a function into the socket queue
func (s *Socket) addListenerIndex(f func(interface{})) int {
	size := s.listeners.Length()
	s.listeners.Add(f)
	return size
}

// //Listen adds a function into the socket queue
// func (s *Socket) Listen(f ...func(interface{})) int {
// 	size := s.listeners.Length()
// 	s.listeners.Add(f...)
// 	return size
// }

//Stream provides a means of piping the data within a channel into
//the sockets channel,ensure to close the chan passed as Stream uses
//range to iterate the channel
func (s *Socket) Stream(data chan interface{}) {
	for k := range data {
		s.Emit(k)
	}
}

//NewSocket returns a new socket instance
func NewSocket(size int) *Socket {
	li := sequence.NewListSequence(make([]interface{}, 0), 20)
	return &Socket{make(chan interface{}, size), li, size}
}

//Pull creates a pull-like socket
type Pull struct {
	*Socket
	pin *Sub
}

//Push creates a push-like socket
type Push struct {
	*Pull
	buffer bool
}

//Emit adds a new data into the channel
func (p *Push) Emit(b interface{}) {
	if !p.buffer {
		if p.Socket.listeners.Length() <= 0 {
			return
		}
	}
	p.Pull.Emit(b)
	p.Pull.Pull()
}

//Close clears the listerns lists and stops listen to parent if existing
func (p *Pull) Close() {
	if p.pin != nil {
		p.pin.Close()
	}
	p.Socket.listeners.Clear()
}

//Pull is called to initiate the pull sequence op
func (p *Pull) Pull() {
	if p.Socket.Size() <= 0 {
		return
	}

	if p.Socket.listeners.Length() <= 0 {
		return
	}

	n := p.Socket.listeners.Iterator()
	data := <-p.Socket.channel

	for n.HasNext() {
		err := n.Next()

		if err != nil {
			break
			// return
		}

		fx, ok := n.Value().(func(interface{}))

		if !ok {
			return
		}

		fx(data)
	}

	p.Pull()
}

//PullSocket returns the socket wrapped up in the Pull struct
func PullSocket(buff int) *Pull {
	return &Pull{NewSocket(buff), nil}
}

//PullSocketWith returns the socket wrapped up in the Pull struct
func PullSocketWith(sock SocketInterface) *Pull {
	su := NewSocket(sock.PoolSize())
	return &Pull{
		su,
		sock.Subscribe(func(v interface{}, _ *Sub) {
			su.Emit(v)
		}),
	}
}

//PushSocket returns the socket wrapped up in the Push struct
func PushSocket(buff int) *Push {
	return &Push{PullSocket(buff), false}
}

//BufferPushSocket returns the socket wrapped up in the Push struct
func BufferPushSocket(buff int) *Push {
	return &Push{PullSocket(buff), true}
}

//BufferPushSocketWith returns the socket wrapped up in the Push struct
func BufferPushSocketWith(sock SocketInterface) *Push {
	return &Push{PullSocketWith(sock), true}
}

//PushSocketWith returns the socket wrapped up in the Push struct
func PushSocketWith(sock SocketInterface) *Push {
	return &Push{PullSocketWith(sock), false}
}

//DoBufferPushSocket creates a pull socket based on a condition
func DoBufferPushSocket(sock SocketInterface, fn func(f interface{}, sock SocketInterface)) *Push {
	su := NewSocket(sock.PoolSize())

	pl := &Pull{su, nil}
	ps := &Push{pl, true}

	pl.pin = sock.Subscribe(func(v interface{}, _ *Sub) {
		fn(v, ps)
	})

	return ps
}

//DoPushSocket creates a pull socket based on a condition
func DoPushSocket(sock SocketInterface, fn func(f interface{}, sock SocketInterface)) *Push {
	su := NewSocket(sock.PoolSize())

	pl := &Pull{su, nil}
	ps := &Push{pl, false}

	pl.pin = sock.Subscribe(func(v interface{}, _ *Sub) {
		fn(v, ps)
	})

	return ps
}

//DoPullSocket creates a pull socket based on a condition
func DoPullSocket(sock SocketInterface, fn func(f interface{}, sock SocketInterface)) *Pull {
	su := NewSocket(sock.PoolSize())

	pl := &Pull{su, nil}

	pl.pin = sock.Subscribe(func(v interface{}, _ *Sub) {
		fn(v, pl)
	})

	return pl
}
