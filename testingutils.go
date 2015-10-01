package flux

import (
	"reflect"
	"testing"
)

// Equaler defines a interface for handling equality
type Equaler interface {
	Equal(Equaler) bool
}

// Expect uses reflect.DeepEqual to evaluate the equality of two values unless the objects both satisfy the Equaler interface
func Expect(t *testing.T, v, m interface{}) {
	vt, vok := v.(Equaler)
	mt, mok := m.(Equaler)

	var state bool
	if vok && mok {
		state = vt.Equal(mt)
	} else {
		state = reflect.DeepEqual(v, m)
	}

	if state {
		FatalFailed(t, "Value %+v and %+v are not a match", v, m)
		return
	}
	LogPassed(t, "Value %+v and %+v are a match", v, m)
}

// StrictExpect uses == to evaluate the equality of two values unless the interface matches the Equaler interface
func StrictExpect(t *testing.T, v, m interface{}) {
	vt, vok := v.(Equaler)
	mt, mok := m.(Equaler)

	var state bool
	if vok && mok {
		state = vt.Equal(mt)
	} else {
		state = (v == m)
	}

	if state {
		FatalFailed(t, "Value %+v and %+v are not a match", v, m)
		return
	}
	LogPassed(t, "Value %+v and %+v are a match", v, m)
}

// Truthy expects a true return value always
func Truthy(t *testing.T, name string, v bool) {
	if !v {
		FatalFailed(t, "Expected truthy value for %s", name)
	} else {
		LogPassed(t, "%s passed with truthy value", name)
	}
}

// Falsy expects a true return value always
func Falsy(t *testing.T, name string, v bool) {
	if !v {
		LogPassed(t, "%s passed with falsy value", name)
	} else {
		FatalFailed(t, "Expected falsy value for %s", name)
	}
}
