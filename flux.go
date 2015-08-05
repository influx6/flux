package flux

import (
	"fmt"
	"sync"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("flux")

//Eachfunc defines the type of the Mappable.Each rule
type Eachfunc func(interface{}, interface{}, func())

//Mappable defines member function rules for securemap
type Mappable interface {
	Clear()
	HasMatch(k, v interface{}) bool
	Each(f Eachfunc)
	Keys() []interface{}
	Copy(map[interface{}]interface{})
	CopySecureMap(Mappable)
	Has(interface{}) bool
	Get(interface{}) interface{}
	Remove(interface{})
	Set(k, v interface{})
	Clone() Mappable
}

//SecureMap simple represents a map with a rwmutex locked in
type SecureMap struct {
	data map[interface{}]interface{}
	lock *sync.RWMutex
}

//Clear unlinks the previous map
func (m *SecureMap) Clear() {
	m.lock.Lock()
	m.data = make(map[interface{}]interface{})
	m.lock.Unlock()
}

//HasMatch checks if a key exists and if the value matches
func (m *SecureMap) HasMatch(key, value interface{}) bool {
	m.lock.RLock()
	k, ok := m.data[key]
	m.lock.RUnlock()

	if ok {
		return k == value
	}

	return false
}

//Each interates through the map
func (m *SecureMap) Each(fn Eachfunc) {
	stop := false
	m.lock.RLock()
	for k, v := range m.data {
		if stop {
			break
		}

		fn(v, k, func() { stop = true })
	}
	m.lock.RUnlock()
}

//Keys return the keys of the map
func (m *SecureMap) Keys() []interface{} {
	m.lock.RLock()
	keys := make([]interface{}, len(m.data))
	count := 0
	for k := range m.data {
		keys[count] = k
		count++
	}
	m.lock.RUnlock()

	return keys
}

//Clone makes a clone for this securemap
func (m *SecureMap) Clone() Mappable {
	sm := NewSecureMap()
	m.lock.RLock()
	for k, v := range m.data {
		sm.Set(k, v)
	}
	m.lock.RUnlock()
	return sm
}

//CopySecureMap Copies a  into the map
func (m *SecureMap) CopySecureMap(src Mappable) {
	src.Each(func(k, v interface{}, _ func()) {
		m.Set(k, v)
	})
}

//Copy Copies a map[interface{}]interface{} into the map
func (m *SecureMap) Copy(src map[interface{}]interface{}) {
	for k, v := range src {
		m.Set(k, v)
	}
}

//Has returns true/false if value exists by key
func (m *SecureMap) Has(key interface{}) bool {
	m.lock.RLock()
	_, ok := m.data[key]
	m.lock.RUnlock()
	return ok
}

//Get a key's value
func (m *SecureMap) Get(key interface{}) interface{} {
	m.lock.RLock()
	k := m.data[key]
	m.lock.RUnlock()
	return k
}

//Set a key with value
func (m *SecureMap) Set(key, value interface{}) {

	if _, ok := m.data[key]; ok {
		return
	}

	m.lock.Lock()
	m.data[key] = value
	m.lock.Unlock()
}

//Remove a value by its key
func (m *SecureMap) Remove(key interface{}) {
	m.lock.Lock()
	delete(m.data, key)
	m.lock.Unlock()
}

//NewSecureMap returns a new securemap
func NewSecureMap() *SecureMap {
	return &SecureMap{make(map[interface{}]interface{}), new(sync.RWMutex)}
}

//SecureMapFrom returns a new securemap
func SecureMapFrom(core map[interface{}]interface{}) *SecureMap {
	return &SecureMap{core, new(sync.RWMutex)}
}

//SecureStack provides addition of functions into a stack
type SecureStack struct {
	listeners []interface{}
	lock      *sync.RWMutex
}

//Splice returns a new splice from the list
func (f *SecureStack) Splice(begin, end int) []interface{} {
	size := f.Size()

	if end > size {
		end = size
	}

	f.lock.RLock()
	ms := f.listeners[begin:end]
	f.lock.RUnlock()
	var dup []interface{}
	dup = append(dup, ms...)
	return dup
}

//Set lets you retrieve an item in the list
func (f *SecureStack) Set(ind int, d interface{}) {
	sz := f.Size()

	if ind >= sz {
		return
	}

	if ind < 0 {
		ind = sz - ind
		if ind < 0 {
			return
		}
	}

	f.lock.Lock()
	f.listeners[ind] = d
	f.lock.Unlock()
}

//Get lets you retrieve an item in the list
func (f *SecureStack) Get(ind int) interface{} {
	sz := f.Size()
	if ind >= sz {
		ind = sz - 1
	}

	if ind < 0 {
		ind = sz - ind
		if ind < 0 {
			return nil
		}
	}

	f.lock.RLock()
	r := f.listeners[ind]
	f.lock.RUnlock()
	return r
}

//Strings return the stringified version of the internal list
func (f *SecureStack) String() string {
	f.lock.RLock()
	sz := fmt.Sprintf("%+v", f.listeners)
	f.lock.RUnlock()
	return sz
}

//Clear flushes the stack listener
func (f *SecureStack) Clear() {
	f.lock.Lock()
	f.listeners = f.listeners[0:0]
	f.lock.Unlock()
}

//Size returns the total number of listeners
func (f *SecureStack) Size() int {
	f.lock.RLock()
	sz := len(f.listeners)
	f.lock.RUnlock()
	return sz
}

//Add adds a function into the stack
func (f *SecureStack) Add(fx interface{}) int {
	f.lock.Lock()
	ind := len(f.listeners)
	f.listeners = append(f.listeners, fx)
	f.lock.Unlock()
	return ind
}

//Delete removes the function at the provided index
func (f *SecureStack) Delete(ind int) {

	if ind <= 0 && len(f.listeners) <= 0 {
		return
	}

	f.lock.RLock()
	copy(f.listeners[ind:], f.listeners[ind+1:])
	f.listeners[len(f.listeners)-1] = nil
	f.listeners = f.listeners[:len(f.listeners)-1]
	f.lock.RUnlock()

}

//Each runs through the function lists and executing with args
func (f *SecureStack) Each(fx func(interface{})) {
	if f.Size() <= 0 {
		return
	}

	f.lock.RLock()
	for _, d := range f.listeners {
		fx(d)
	}
	f.lock.RUnlock()
}

//FunctionStack provides addition of functions into a stack
type FunctionStack struct {
	listeners []func(...interface{})
	lock      *sync.RWMutex
}

//Clear flushes the stack listener
func (f *FunctionStack) Clear() {
	f.lock.Lock()
	f.listeners = f.listeners[0:0]
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
	f.lock.Lock()
	ind := len(f.listeners)
	f.listeners = append(f.listeners, fx)
	f.lock.Unlock()
	return ind
}

//Delete removes the function at the provided index
func (f *FunctionStack) Delete(ind int) {

	if ind <= 0 && len(f.listeners) <= 0 {
		return
	}

	f.lock.RLock()
	copy(f.listeners[ind:], f.listeners[ind+1:])
	f.listeners[len(f.listeners)-1] = nil
	f.listeners = f.listeners[:len(f.listeners)-1]
	f.lock.RUnlock()

}

//Each runs through the function lists and executing with args
func (f *FunctionStack) Each(d ...interface{}) {
	if f.Size() <= 0 {
		return
	}

	f.lock.RLock()
	for _, fx := range f.listeners {
		fx(d...)
	}
	f.lock.RUnlock()
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

//NewSecureStack returns a new functionstack instance
func NewSecureStack() *SecureStack {
	return &SecureStack{
		make([]interface{}, 0),
		new(sync.RWMutex),
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
