package flux

import "testing"

func TestReactiveStack(t *testing.T) {

	gen := ReactIdentity()

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
