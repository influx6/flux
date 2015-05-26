package flux

import (
	"runtime"
	"sync"
)

//SocketInterface defines member function rules
type SocketInterface interface {
	Emit(interface{})
	Stream(chan interface{})
	addListenerIndex(func(interface{})) int
	removeListenerIndex(int)
	Subscribe(func(interface{}, *Sub)) *Sub
	Size() int
	PoolSize() int
	Close()
	ClearListeners()
	ListenerSize() int
}

//Pipe represent the interface that defines the behaviour of Push and any other
//type build on top of the SocketInterface type
type Pipe interface {
	SocketInterface
	PushStream()
	Wait()
}

//Sub provides a nice clean subscriber connection for socket
type Sub struct {
	socket  SocketInterface
	pin     int
	fnx     func(interface{}, *Sub)
	removed bool
}

//Close disconnects the subscription
func (s *Sub) Close() {
	if s.socket == nil {
		return
	}
	s.socket = nil
	s.removed = true
	// s.socket.removeListenerIndex(s.pin)
	s.pin = 0
}

//NewSub returns a new subscriber
func NewSub(sock SocketInterface, fn func(interface{}, *Sub)) *Sub {
	sd := &Sub{sock, -1, fn, false}
	sd.pin = sock.addListenerIndex(func(v interface{}) {
		if !sd.removed {
			sd.fnx(v, sd)
		}
	})
	return sd
}

//Socket is the base structure for all data flow communication
type Socket struct {
	channel    chan interface{}
	closer     chan struct{}
	listeners  *SingleStack
	bufferSize int
	bufferup   bool
	when       *sync.Once
}

//PoolSize returns the size of data in the channel
func (s *Socket) PoolSize() int {
	return s.bufferSize
}

//ClearListeners clears the sockets listener list
func (s *Socket) ClearListeners() {
	s.listeners.Clear()
}

//Close closes the socket internal channel and clears its listener list
func (s *Socket) Close() {
	close(s.channel)
	<-s.closer
	// s.listeners.lock.Lock()
	s.ClearListeners()
	// s.listeners.lock.Unlock()
}

//Subscribe returns a subscriber
func (s *Socket) Subscribe(fx func(interface{}, *Sub)) *Sub {
	// if s.bufferup {
	// 	close(s.begin)
	// }
	return NewSub(s, fx)
}

//ListenerSize returns the size of listeners
func (s *Socket) ListenerSize() int {
	return s.listeners.Size()
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
	return s.listeners.Add(f)
}

//Stream provides a means of piping the data within a channel into
//the sockets channel,ensure to close the chan passed as Stream uses
//range to iterate the channel
func (s *Socket) Stream(data chan interface{}) {
	for k := range data {
		s.Emit(k)
	}
}

//NewSocket returns a new socket instance
func NewSocket(size int, buf bool) *Socket {
	li := NewSingleStack()
	return &Socket{
		make(chan interface{}, size),
		make(chan struct{}),
		li,
		size,
		buf,
		new(sync.Once),
	}
}

//Push creates a push-like socket
type Push struct {
	*Socket
	pin  *Sub
	wait *sync.WaitGroup
	// closer chan struct{}
}

//Pull creates a pull-like socket
type Pull struct {
	*Push
}

//Emit adds a new data into the channel
func (p *Pull) Emit(b interface{}) {
	if !p.bufferup {
		if p.Socket.ListenerSize() <= 0 {
			return
		}
	}

	size := p.Push.Size()
	max := cap(p.Push.Socket.channel)

	if size >= max {
		<-p.Push.Socket.channel
	}

	p.Push.Emit(b)
}

//Emit adds a new data into the channel
func (p *Push) Emit(b interface{}) {
	if !p.bufferup {
		if p.Socket.ListenerSize() <= 0 {
			return
		}
	}
	p.Socket.Emit(b)
}

//Close clears the listerns lists and stops listen to parent if existing
func (p *Push) Close() {
	if p.pin != nil {
		p.pin.Close()
	}
	p.Socket.Close()
}

//Wait caues the socket to wait till its done
func (p *Push) Wait() {
	p.wait.Wait()
}

//PushStream uses the range iterator over the terminal
func (p *Pull) PushStream() {
	size := p.Push.Size()

	if size > 0 {
		p.wait.Add(1)
		dx := <-p.Push.Socket.channel
		p.listeners.Each(dx)
		p.wait.Done()
		runtime.Gosched()
		p.PushStream()
	}
}

//PushStream uses the range iterator over the terminal
func (p *Push) PushStream() {
	p.wait.Add(1)
	go func() {
		// <-p.begin
		for dx := range p.Socket.channel {
			p.listeners.Each(dx)
			runtime.Gosched()
		}
		p.wait.Done()
		p.when.Do(func() {
			close(p.closer)
		})
	}()
}

//PullSocket returns the socket wrapped up in the Push struct
func PullSocket(buff int) *Pull {
	if buff <= 0 {
		buff = 1
	}

	ps := &Pull{
		&Push{
			NewSocket(buff, false),
			nil,
			new(sync.WaitGroup),
		},
	}

	ps.when.Do(func() {
		close(ps.closer)
	})

	return ps
}

//PushSocket returns the socket wrapped up in the Push struct
func PushSocket(buff int) *Push {
	ps := &Push{
		NewSocket(buff, false),
		nil,
		new(sync.WaitGroup),
	}
	ps.PushStream()
	return ps
}

//PushSocketWith returns the socket wrapped up in the Push struct
func PushSocketWith(sock SocketInterface) *Push {
	su := NewSocket(sock.PoolSize(), false)
	ps := &Push{
		su,
		sock.Subscribe(func(v interface{}, _ *Sub) {
			su.Emit(v)
		}),
		new(sync.WaitGroup),
	}

	ps.PushStream()
	return ps
}

//PullSocketWith creates a pull socket based on a condition
func PullSocketWith(size int, sock SocketInterface) *Pull {
	su := NewSocket(size, false)
	ps := &Pull{
		&Push{
			su,
			sock.Subscribe(func(v interface{}, _ *Sub) {
				su.Emit(v)
			}),
			new(sync.WaitGroup),
		},
	}

	ps.when.Do(func() {
		close(ps.closer)
	})
	return ps
}

//DoPushSocket creates a push socket based on a condition
func DoPushSocket(sock SocketInterface, fn func(f interface{}, sock SocketInterface)) *Push {
	su := NewSocket(sock.PoolSize(), false)
	ps := &Push{
		su,
		nil,
		new(sync.WaitGroup),
	}

	ps.pin = sock.Subscribe(func(v interface{}, _ *Sub) {
		fn(v, ps)
	})

	ps.PushStream()
	return ps
}

//DoPullSocket creates a pull socket based on a condition
func DoPullSocket(size int, sock SocketInterface, fn func(f interface{}, sock SocketInterface)) *Pull {
	su := NewSocket(size, false)
	ps := &Pull{
		&Push{
			su,
			nil,
			new(sync.WaitGroup),
		},
	}

	ps.when.Do(func() {
		close(ps.closer)
	})

	ps.pin = sock.Subscribe(func(v interface{}, _ *Sub) {
		fn(v, ps)
	})

	return ps
}
