package flux

import (
	"sync"
	"testing"
)

func TestReactiveStack(t *testing.T) {

	ws := new(sync.WaitGroup)
	gen := ReactIdentity()

	ws.Add(2)
	mn := gen.React(func(k ReactiveStacks) {
		func() {
		mloop:
			for {
				select {
				case <-k.In():
					ws.Done()
				case <-k.Feed().Out():
					ws.Done()
				case <-gen.Closed():
					k.End()
					break mloop
				}
			}
		}()
	})

	gen.In() <- 2
	mn.In() <- 40

	ws.Wait()
	gen.End()

}
