package flux

import (
	"errors"
	"testing"
)

func TestActDepend(t *testing.T) {
	ax := NewAction()
	ag := NewActDepend(ax, 3)

	_ = ag.MeddleBefore(1, func(b interface{}, next ActionInterface) {
		t.Log("1 got:", b)
		next.Fullfill(b)
	})

	ag.Then(func(b interface{}, next ActionInterface) {
		t.Log("first :", b)
		next.Fullfill("through!")
	}).Then(func(b interface{}, next ActionInterface) {
		t.Log("scond then:", b)
		next.Fullfill(b)
	}).Then(func(b interface{}, next ActionInterface) {
		t.Log("third then:", b)
		next.Fullfill(b)
	}).Then(func(b interface{}, next ActionInterface) {
		t.Log("third then:", b)
		next.Fullfill(b)
	})

	ax.Fullfill("Sounds!")
}

func TestAction(t *testing.T) {
	ax := NewAction()

	av := ax.Then(func(v interface{}, a ActionInterface) {
		_, ok := v.(string)

		if !ok {
			a.Fullfill(false)
			return
		}

		a.Fullfill(true)
	}).When(func(v interface{}, _ ActionInterface) {

		state, _ := v.(bool)

		if !state {
			t.Fatal("Fullfilled value is not a string")
		}

		t.Log("we got a string:", state)
	})

	t.Log("final action:", av)

	ax.Fullfill("Sounds!")
}

func TestSuccessActionStack(t *testing.T) {
	// var _ interface{}
	ax := NewActionStack()

	_ = ax.Done().When(func(v interface{}, _ ActionInterface) {
		t.Log("we passed:", v)
	})

	_ = ax.Error().When(func(v interface{}, _ ActionInterface) {
		t.Log("we failed:", v)
	})

	ax.Complete("Sounds!")
}

func TestFailedActionStack(t *testing.T) {
	// var _ interface{}
	ax := NewActionStack()

	_ = ax.Done().When(func(v interface{}, _ ActionInterface) {
		t.Log("we passed:", v)
	})

	_ = ax.Error().When(func(v interface{}, _ ActionInterface) {
		t.Log("we failed:", v)
	}).Then(func(v interface{}, b ActionInterface) {
		t.Log("we failed again:", v)
		b.Fullfill(v)
	})

	ax.Complete(errors.New("1000"))
}
