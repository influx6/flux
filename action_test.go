package flux

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestTimeWaiter(t *testing.T) {
	ms := time.Duration(1) * time.Second

	w := NewTimeWait(0, ms)
	ws := new(sync.WaitGroup)

	ws.Add(1)

	w.Then().When(func(v interface{}, _ ActionInterface) {
		_, ok := v.(int)

		if !ok {
			t.Fatal("Waiter completed with non-int value:", v)
		}

		t.Log("TimeWaiter Finished")
		ws.Done()
	})

	if w == nil {
		t.Fatal("Unable to create waiter")
	}

	if w.Count() < 0 {
		t.Fatal("Waiter completed before use")
	}

	t.Log("Before Add: Count:", w.Count())
	w.Add()
	w.Add()
	t.Log("After 2 Add: Count:", w.Count())

	if w.Count() < 1 {
		t.Fatalf("TimeWaiter count is at %d below 1 after two calls to Add()", w.Count())
	}

	if w.Count() < -1 {
		t.Fatal("Waiter just did a bad logic and went below -1")
	}

	// w.Done()

	ws.Wait()
}

func TestWaiter(t *testing.T) {
	w := NewWait()

	w.Then().When(func(v interface{}, _ ActionInterface) {
		_, ok := v.(int)

		if !ok {
			t.Fatal("Waiter completed with non-int value:", v)
		}

		t.Log("Waiter Finished")
	})

	if w == nil {
		t.Fatal("Unable to create waiter")
	}

	if w.Count() < 0 {
		t.Fatal("Waiter completed before use")
	}

	w.Add()

	if w.Count() < 1 {
		t.Fatalf("Waiter count is still at %v despite call to Add()", w.Count())
	}

	cu := w.Count()

	w.Done()

	if w.Count() > 1 {
		t.Fatalf("Waiter count (%v) is still greater (%v) despite call to Done()", w.Count(), cu)
	}

	w.Done()
	w.Done()

	if w.Count() < -1 {
		t.Fatal("Waiter just did a bad logic and went below -1")
	}
}

func TestActDepend(t *testing.T) {
	ax := NewAction()
	ag := NewActDepend(ax, 3)

	_ = ag.OverrideBefore(1, func(b interface{}, next ActionInterface) {
		next.Fullfill(b)
	})

	fx := ag.Then(func(b interface{}, next ActionInterface) {
		next.Fullfill("through!")
	}).Then(func(b interface{}, next ActionInterface) {
		next.Fullfill(b)
	}).Then(func(b interface{}, next ActionInterface) {
		next.Fullfill("josh")
	})

	ax.Fullfill("Sounds!")

	fv := <-fx.Sync(20)
	if "josh" != fv {
		t.Log("Final value is wrong", fv, fx, ag)
	}

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
