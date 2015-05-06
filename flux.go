package flux

import (
	"sync"
	"time"
)

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
	Chain(int) *ActDepend
	ChainWith(...ActionInterface) *ActDepend
	Wrap() *ActionWrap
	Sync(int) <-chan interface{}
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

//Chain returns ActDepend(ActionDepend) with this action as the root
func (a *ActionWrap) Chain(m int) *ActDepend {
	return a.action.Chain(m)
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

//ChainWith returns ActDepend(ActionDepend) with this action as the root
func (a *ActionWrap) ChainWith(r ...ActionInterface) *ActDepend {
	return a.action.ChainWith(r...)
}

//Sync returns unbuffered channel which will get resolved with the
//value of the action when fullfilled
func (a *ActionWrap) Sync(ms int) <-chan interface{} {
	return a.action.Sync(ms)
}

//Action provides a future-style connect approach
type Action struct {
	fired bool
	stack *SingleStack
	cache interface{}
}

//Sync returns unbuffered channel which will get resolved with the
//value of the action when fullfilled or when the supplied value of
//time has passed it will eject
func (a *Action) Sync(ms int) <-chan interface{} {
	m := make(chan interface{})

	if ms <= 0 {
		ms = 1
	}

	md := time.Duration(ms) * time.Millisecond
	closed := false

	a.When(func(b interface{}, _ ActionInterface) {
		go func() {
			m <- b
			if !closed {
				closed = true
				close(m)
			}
		}()
	})

	go func() {
		<-time.After(md)
		if !closed {
			close(m)
		}
	}()
	return m
}

//Chain returns ActDepend(ActionDepend) with this action as the root
func (a *Action) Chain(max int) *ActDepend {
	return NewActDepend(a, max)
}

//ChainWith returns ActDepend(ActionDepend) with this action as the root
func (a *Action) ChainWith(r ...ActionInterface) *ActDepend {
	return NewActDependWith(a, r...)
}

//ActDepend provides a nice means of creating a new action depending on
//unfullfilled action
type ActDepend struct {
	root    ActionInterface
	waiters []ActionInterface
	states  map[int]bool
	ind     int
	max     int
	ended   bool
}

//NewActDepend returns a action resolver based on a root action,when this root
//action is resolved,it waits on the user to call the actdepend then method to complete
//the next action,why so has to allow user-based chains where the user must partake in the
//completion of the final action
func NewActDepend(r ActionInterface, max int) *ActDepend {
	var set = make([]ActionInterface, max)
	var count = 0

	act := &ActDepend{
		r,
		set,
		make(map[int]bool),
		0,
		max,
		false,
	}

	for count < max {
		m := NewAction()
		c := count
		m.When(func(b interface{}, _ ActionInterface) {
			act.states[c] = true
		})
		set[count] = m
		act.states[count] = false
		count++
	}

	return act
}

//NewActDependWith provides the actdepend struct but allows specifying the next call in the chan
func NewActDependWith(root ActionInterface, r ...ActionInterface) *ActDepend {
	var max = len(r)

	act := &ActDepend{
		root,
		r,
		make(map[int]bool),
		0,
		max,
		false,
	}

	for k, m := range r {
		act.states[k] = false
		c := k
		m.When(func(b interface{}, _ ActionInterface) {
			act.states[c] = false
		})
	}

	return act
}

//NewActDependBy provides the actdepend struct but allows specifying the next call in the chan
func NewActDependBy(r ActionInterface, v ActionInterface, max int) *ActDepend {
	var set = make([]ActionInterface, max)
	var count = 1

	act := &ActDepend{
		r,
		set,
		make(map[int]bool),
		0,
		max,
		false,
	}

	for count < max {
		m := NewAction()
		c := count
		m.When(func(b interface{}, _ ActionInterface) {
			act.states[c] = false
		})
		set[count] = m
		act.states[count] = false
		count++
	}

	return act
}

func (a *ActDepend) correctIndex(index int) (int, bool) {
	ind := 0

	if index < 0 {
		ind = a.Size() - ind
		if ind < 0 || ind > a.Size() {
			return 0, false
		}
	} else {
		ind = index
		if ind >= a.Size() {
			ind--
		}
	}

	return ind, true

}

//Use returns the ActionInterface wrapped by an ActionWrap
//at the index or nil
//supports negative indexing
func (a *ActDepend) Use(ind int) ActionInterface {
	return a.getIndex(ind).Wrap()
}

//GetIndex returns the ActionInterface at the index or nil
//supports negative indexing
func (a *ActDepend) getIndex(ind int) ActionInterface {
	ind, ok := a.correctIndex(ind)

	if !ok {
		return nil
	}

	return a.waiters[ind]
}

//IsIndexFullfilled returns true/false if the action at the index is fullfilled
func (a *ActDepend) IsIndexFullfilled(ind int) bool {
	return a.getIndex(ind).Fullfilled()
}

//OverrideAfter allows calling Then with an action after the current index
//that is you want to listen to the action at this index to fullfill the
//next index
func (a *ActDepend) OverrideAfter(index int, fx func(b interface{}, a ActionInterface)) ActionInterface {
	ind, ok := a.correctIndex(index)

	if !ok {
		return a
	}

	ax := a.waiters[ind]
	var dx ActionInterface
	var nx = ind + 1

	if _, ok := a.states[nx]; ok {
		a.states[nx] = true
	}

	if nx >= a.Size() {
		return ax.Then(fx)
	}

	dx = a.waiters[nx]

	return ax.UseThen(fx, dx)

}

//OverrideBefore allows calling Then with an action before the current index
//that is you want to listen to the action at this previous index to fullfill the
//this action at this index
func (a *ActDepend) OverrideBefore(index int, fx func(b interface{}, a ActionInterface)) ActionInterface {
	ind, ok := a.correctIndex(index)

	if !ok {
		return a
	}

	ax := a.waiters[ind]

	var dx ActionInterface

	if ind < 1 {
		dx = a.root
	} else {
		dx = a.waiters[ind-1]
	}

	if _, ok := a.states[ind]; ok {
		a.states[ind] = true
	}

	return dx.UseThen(fx, ax)
}

//Shift pushes the index up if the action is not yet fully ended
//allows the next call of Then to be shifted as this states the user
//wants to manage this on their own
func (a *ActDepend) shift() {
	if a.ended {
		return
	}

	if a.ind >= a.Size() {
		return
	}

	a.ind++
}

//Size returns the total actions in list
func (a *ActDepend) Size() int {
	return len(a.waiters)
}

//current gets the last ActionInterface in the chain
func (a *ActDepend) current() ActionInterface {
	return a.waiters[a.ind]
}

//ChainWith returns ActDepend(ActionDepend) with this action as the root
func (a *ActDepend) ChainWith(r ...ActionInterface) *ActDepend {
	return a.current().ChainWith(r...)
}

//Sync returns unbuffered channel which will get resolved with the
//value of the action when fullfilled
func (a *ActDepend) Sync(ms int) <-chan interface{} {
	return a.current().Sync(ms)
}

//End stops the generation of new chain
func (a *ActDepend) End() {
	a.ended = true
}

//Chain returns ActDepend(ActionDepend) with this action as the root
func (a *ActDepend) Chain(max int) *ActDepend {
	return a
}

//Wrap returns actionwrap for the action
func (a *ActDepend) Wrap() *ActionWrap {
	return a.current().Wrap()
}

//Fullfilled returns true or false if the action is done
func (a *ActDepend) Fullfilled() bool {
	return a.current().Fullfilled()
}

//Fullfill actually fullfills the root action if its not fullfilled already
func (a *ActDepend) Fullfill(b interface{}) {
	a.root.Fullfill(b)
}

//When adds a function to the action stack with the action as the second arg
func (a *ActDepend) When(fx func(b interface{}, a ActionInterface)) ActionInterface {
	return a.current().When(fx)
}

//Then adds a function to the action stack or fires immediately if done
func (a *ActDepend) Then(fx func(b interface{}, a ActionInterface)) ActionInterface {
	ind := a.ind
	sz := a.Size()

	if a.ended {
		return a.waiters[sz-1].Then(fx)
	}

	cur := a.current()

	if a.states[ind] {
		a.ind++
		return a.Then(fx)
	}

	if ind <= 0 {
		_ = a.root.UseThen(fx, cur)
	} else {
		act := a.waiters[a.ind-1]
		act.UseThen(fx, cur)
	}

	a.states[ind] = true

	if a.ind < sz && !(a.ind+1 >= sz) {
		a.ind++
	} else {
		a.ended = true
	}

	return a
}

//UseThen adds a function with a ActionInterface to the action stack or fires immediately if done
//once done that action interface is returned
func (a *ActDepend) UseThen(fx func(b interface{}, a ActionInterface), f ActionInterface) ActionInterface {
	return a.current().UseThen(fx, a)
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
	success ActionInterface
	failed  ActionInterface
}

//Done returns the action for the done state
func (a *ActionStack) Done() ActionInterface {
	return a.success
}

//Error returns the action for the error state
func (a *ActionStack) Error() ActionInterface {
	return a.failed
}

//Complete allows completion of an action stack
func (a *ActionStack) Complete(b interface{}) ActionInterface {
	if a.failed.Fullfilled() {
		return a.failed.Wrap()
	}

	if a.success.Fullfilled() {
		return a.success.Wrap()
	}

	e, ok := b.(error)

	if ok {
		a.failed.Fullfill(e)
		return a.failed.Wrap()
	}

	a.success.Fullfill(b)
	return a.success.Wrap()
}

//NewActionStack returns a new actionStack
func NewActionStack() *ActionStack {
	return &ActionStack{
		NewAction(),
		NewAction(),
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
	}
}

//NewActionStackBy returns a new actionstack with the predefined actions
//from a previous actionstack with modification
func NewActionStackBy(d ActionInterface, e ActionInterface) *ActionStack {
	return &ActionStack{
		d,
		e,
	}
}
