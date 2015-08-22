package flux

import "testing"

func TestReactiveStack(t *testing.T) {

	gen := Reactive(func(r ReactiveStacks) {
	ml:
		for {
			select {
			case c := <-r.In():
				r.Out() <- c.(int)
			case <-r.Closed():
				break ml
			}
		}
	}, nil)

	go func() {
	mloop:
		for {
			select {
			case data := <-gen.Out():
				t.Logf("data: %v", data)
			case <-gen.Closed():
				break mloop
			}
		}
	}()

	gen.In() <- 2

	gen.End()

}
