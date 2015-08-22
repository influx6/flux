package flux

import (
	"sync"
	"testing"
)

//TestBasicReaction provides a test case for a simple two-way reaction i.e receive and react infinitely
//to data being sent into a pipe until that pipe is closed
func TestBasicReaction(t *testing.T) {

	ws := new(sync.WaitGroup)

	//ReactIdentity returns reactor that simply pipes what input it gets to its output if it has a connection

	//collect is the root reactor
	collect := ReactIdentity()

	ws.Add(2)

	//.React() is the simple approach of branching out from a root reactor,you receive the new reactor its self
	//but are able to get the parent reactor using the .Feed() function and collect the data from its Out() pipe
	//all reactor provide a .Close() channel that lets you listen in for when to stop operations,this way you get to decide
	//if you wish to stop the new reactor along
	mn := collect.React(func(k ReactiveStacks) {
		func() {
		mloop:
			for {
				select {
				case <-k.In():
					ws.Done()
				case <-k.Feed().Out():
					ws.Done()
				case <-k.Feed().Closed():
					k.End()
					break mloop
				}
			}
		}()
	})

	/*we can emit/send data for reaction using the in-channel returned bythe In() function*/
	collect.In() <- 2 //sends data into the root reactor
	mn.In() <- 40     //sends data only into the current reactor

	ws.Wait()

	//Reactors are closed by calling the .End() function
	collect.End()

}
