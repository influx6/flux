package flux

import (
	"log"
	"sync"
	"testing"
)

// TestBasicReaction provides a test case for a simple two-way reaction i.e receive and react infinitely to data being sent into a pipe until that pipe is closed
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
				case <-k.Closed():
					break mloop
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

	mn.End()
	//Reactors are closed by calling the .End() function
	collect.End()
}

func TestMultiplier(t *testing.T) {
	ws := new(sync.WaitGroup)

	//collect is the root reactor
	mul := ReactIdentity()

	ws.Add(4)

	col := mul.React(ReactReceive())

	GoDefer("Receve1", func() {
	nl:
		for {
			select {
			case data := <-col.Out():
				log.Println("received1:", data)
				ws.Done()
			case <-col.Closed():
				log.Println("closing 1")
				break nl
			}
		}
		// ws.Done()
	})

	col2 := mul.React(ReactReceive())

	GoDefer("Receive2", func() {
	nl:
		for {
			select {
			case data := <-col2.Out():
				log.Println("received2:", data)
				ws.Done()
			case <-col2.Closed():
				log.Println("closing 2")
				break nl
			}
		}
		// ws.Done()
	})

	mul.In() <- 3
	mul.In() <- 40

	// col2.End()
	// col.End()

	// mul.In() <- 50

	ws.Wait()
	mul.End()
}
