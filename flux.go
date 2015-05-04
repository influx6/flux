package flux

import "sync"

//FunctionStack provides addition of functions into a stack
type FunctionStack struct {
	listeners []func(...interface{})
	lock      *sync.RWMutex
}

//Clear flushes the stack listener
func (f *FunctionStack) Clear() {
	f.lock.Lock()
	f.listeners = make([]func(...interface{}), 0)
	f.lock.Unlock()
}

//Size returns the total number of listeners
func (f *FunctionStack) Size() int {
	f.lock.RLock()
	sz := len(f.listeners)
	f.lock.RUnlock()
	return sz
}

//Add adds a function into the stack
func (f *FunctionStack) Add(fx func(...interface{})) int {
	f.lock.RLock()
	ind := len(f.listeners)
	f.listeners = append(f.listeners, fx)
	f.lock.RUnlock()
	return ind
}

//Delete removes the function at the provided index
func (f *FunctionStack) Delete(ind int) {
	f.lock.Lock()

	if ind <= 0 && len(f.listeners) <= 0 {
		return
	}

	copy(f.listeners[ind:], f.listeners[ind+1:])
	f.listeners[len(f.listeners)-1] = nil
	f.listeners = f.listeners[:len(f.listeners)-1]

	f.lock.Unlock()
}

//Each runs through the function lists and executing with args
func (f *FunctionStack) Each(d ...interface{}) {
	// f.lock.RLock()
	for _, fx := range f.listeners {
		fx(d...)
	}
	// f.lock.RUnlock()
}

//SingleStack provides a function stack fro single argument
//functions
type SingleStack struct {
	*FunctionStack
}

//Add adds a function into the stack
func (s *SingleStack) Add(fx func(interface{})) int {
	return s.FunctionStack.Add(func(f ...interface{}) {
		fx(f[0])
	})
}

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
	listeners  *SingleStack
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

//NewFunctionStack returns a new functionstack instance
func NewFunctionStack() *FunctionStack {
	return &FunctionStack{
		make([]func(...interface{}), 0),
		new(sync.RWMutex),
	}
}

//NewSingleStack returns a singlestack instance
func NewSingleStack() *SingleStack {
	return &SingleStack{
		NewFunctionStack(),
	}
}

//NewSocket returns a new socket instance
func NewSocket(size int) *Socket {
	li := NewSingleStack()
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
		if p.Socket.listeners.Size() <= 0 {
			return
		}
	}
	p.Pull.Emit(b)
	p.Pull.PullStream()
}

//Close clears the listerns lists and stops listen to parent if existing
func (p *Pull) Close() {
	if p.pin != nil {
		p.pin.Close()
	}
	p.Socket.listeners.Clear()
}

//PullStream is called to initiate the pull sequence op
func (p *Pull) PullStream() {
	if p.Socket.Size() <= 0 {
		return
	}

	if p.Socket.listeners.Size() <= 0 {
		return
	}

	data := <-p.Socket.channel
	p.listeners.Each(data)
	p.PullStream()
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

//ActionInterface defines member functions
type ActionInterface interface {
	Fullfill(b interface{})
	When(fx func(interface{}, ActionInterface)) ActionInterface
	Then(fx func(interface{}, ActionInterface)) ActionInterface
	UseThen(fx func(interface{}, ActionInterface), a ActionInterface) ActionInterface
	Fullfilled() bool
	Wrap() *ActionWrap
}

//ActionStackInterface defines actionstack member method rules
type ActionStackInterface interface {
	Complete(b interface{}) ActionInterface
	Done() ActionInterface
	Error() ActionInterface
}

//ActionWrap safty wraps action for limited access to its fullfill function
type ActionWrap struct {
	action ActionInterface
}

//NewActionWrap returns a action wrapped in a actionwrap
func NewActionWrap(a *Action) *ActionWrap {
	return &ActionWrap{a}
}

//Fullfilled returns true or false if the action is done
func (a *ActionWrap) Fullfilled() bool {
	return a.action.Fullfilled()
}

//Fullfill meets this action of this structure
func (a *ActionWrap) Fullfill(b interface{}) {
	return
}

//When adds a function to the action stack with the action as the second arg
func (a *ActionWrap) When(fx func(b interface{}, a ActionInterface)) ActionInterface {
	return a.action.When(fx)
}

//Then adds a function to the action stack or fires immediately if done
func (a *ActionWrap) Then(fx func(b interface{}, a ActionInterface)) ActionInterface {
	return a.action.Then(fx)
}

//UseThen adds a function with a ActionInterface to the action stack or fires immediately if done
//once done that action interface is returned
func (a *ActionWrap) UseThen(fx func(b interface{}, a ActionInterface), f ActionInterface) ActionInterface {
	return a.action.UseThen(fx, a)
}

//Wrap returns actionwrap for the action
func (a *ActionWrap) Wrap() *ActionWrap {
	return a
}

//Action provides a future-style connect approach
type Action struct {
	fired bool
	stack *SingleStack
	cache interface{}
}

//ActDepend provides a nice means of creating a new action depending on
//unfullfilled action
type ActDepend struct {
	root   ActionInterface
	waiter ActionInterface
}

//NewActDepend returns a action resolver based on a root action,when this root
//action is resolved,it waits on the user to call the actdepend then method to complete
//the next action,why so has to allow user-based chains where the user must partake in the
//completion of the final action
func NewActDepend(r ActionInterface) *ActDepend {
	return &ActDepend{
		r,
		NewAction(),
	}
}

//NewActDependBy provides the actdepend struct but allows specifying the next call in the chan
func NewActDependBy(r ActionInterface, v ActionInterface) *ActDepend {
	return &ActDepend{
		r,
		v,
	}
}

//Wrap returns actionwrap for the action
func (a *ActDepend) Wrap() *ActionWrap {
	return a.waiter.Wrap()
}

//Fullfilled returns true or false if the action is done
func (a *ActDepend) Fullfilled() bool {
	return a.waiter.Fullfilled()
}

//Fullfill actually fullfills the root action if its not fullfilled already
func (a *ActDepend) Fullfill(b interface{}) {
	a.root.Fullfill(b)
}

//When adds a function to the action stack with the action as the second arg
func (a *ActDepend) When(fx func(b interface{}, a ActionInterface)) ActionInterface {
	return a.waiter.When(fx)
}

//Then adds a function to the action stack or fires immediately if done
func (a *ActDepend) Then(fx func(b interface{}, a ActionInterface)) ActionInterface {
	if a.waiter.Fullfilled() {
		return a.waiter.Then(fx)
	}

	return a.root.UseThen(fx, a.waiter)
}

//UseThen adds a function with a ActionInterface to the action stack or fires immediately if done
//once done that action interface is returned
func (a *ActDepend) UseThen(fx func(b interface{}, a ActionInterface), f ActionInterface) ActionInterface {
	return a.waiter.UseThen(fx, a)
}

//Wrap returns actionwrap for the action
func (a *Action) Wrap() *ActionWrap {
	return NewActionWrap(a)
}

//Fullfilled returns true or false if the action is done
func (a *Action) Fullfilled() bool {
	return a.fired
}

//Fullfill meets this action of this structure
func (a *Action) Fullfill(b interface{}) {
	if !a.fired {
		a.cache = b
		a.stack.Each(b)
		a.fired = true
		a.stack.Clear()
	}
}

//When adds a function to the action stack with the action as the second arg
func (a *Action) When(fx func(b interface{}, a ActionInterface)) ActionInterface {
	if a.fired {
		fx(a.cache, a)
	} else {
		a.stack.Add(func(res interface{}) {
			// log.Println("calling when callback:", res, a)
			fx(res, a)
			// log.Println("called when!")
		})
	}

	return a
}

//Then adds a function to the action stack or fires immediately if done
func (a *Action) Then(fx func(b interface{}, a ActionInterface)) ActionInterface {
	ac := NewAction()
	if a.fired {
		fx(a.cache, ac)
	} else {
		a.stack.Add(func(res interface{}) {
			fx(res, ac)
		})
	}

	return ac
}

//UseThen adds a function with a ActionInterface to the action stack or fires immediately if done
//once done that action interface is returned
func (a *Action) UseThen(fx func(b interface{}, a ActionInterface), f ActionInterface) ActionInterface {
	if a.fired {
		fx(a.cache, f)
	} else {
		a.stack.Add(func(res interface{}) {
			fx(res, f)
		})
	}

	return f
}

//NewAction returns a new Action struct
func NewAction() *Action {
	return &Action{false, NewSingleStack(), nil}
}

//ActionStack provides two internal stack for success and error
type ActionStack struct {
	done    ActionInterface
	errord  ActionInterface
	doned   ActionInterface
	errored ActionInterface
}

//Done returns the action for the done state
func (a *ActionStack) Done() ActionInterface {
	return a.doned
}

//Error returns the action for the error state
func (a *ActionStack) Error() ActionInterface {
	return a.errored
}

//Complete allows completion of an action stack
func (a *ActionStack) Complete(b interface{}) ActionInterface {
	if a.errord.Fullfilled() {
		return a.errord
	}

	if a.done.Fullfilled() {
		return a.done
	}

	e, ok := b.(error)

	if ok {
		a.errord.Fullfill(e)
		return a.errord
	}

	a.done.Fullfill(b)
	return a.done
}

//NewActionStack returns a new actionStack
func NewActionStack() *ActionStack {
	d := NewAction()
	e := NewAction()

	return &ActionStack{
		d,
		e,
		d.Wrap(),
		e.Wrap(),
	}
}

//ActionMod defines a function type that modifies a actionstack actions
//and returns them or the new actions
type ActionMod func(a ActionStackInterface) (ActionInterface, ActionInterface)

//NewActionStackFrom returns a new actionstack with the predefined actions
//from a previous actionstack with modification
func NewActionStackFrom(a ActionStackInterface, mod ActionMod) *ActionStack {
	d, e := mod(a)
	return &ActionStack{
		d,
		e,
		d.Wrap(),
		e.Wrap(),
	}
}

//NewActionStackBy returns a new actionstack with the predefined actions
//from a previous actionstack with modification
func NewActionStackBy(d ActionInterface, e ActionInterface) *ActionStack {
	return &ActionStack{
		d,
		e,
		d.Wrap(),
		e.Wrap(),
	}
}
