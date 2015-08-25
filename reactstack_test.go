package flux

import (
	"errors"
	"sync"
	"testing"
)

// TestBasicReaction provides a test case for a simple two-way reaction i.e receive and react infinitely to data being sent into a pipe until that pipe is closed
func TestBasicReaction(t *testing.T) {

	ws := new(sync.WaitGroup)

	//ReactIdentity returns reactor that simply pipes what input it gets to its output if it has a connection

	//collect is the root reactor
	collect := ReactIdentity()

	ws.Add(3)

	//.React() is the simple approach of branching out from a root reactor,you receive the new reactor its self
	//but are able to get the parent reactor using the .Feed() function and collect the data from its Out() pipe
	//all reactor provide a .Close() channel that lets you listen in for when to stop operations,this way you get to decide
	//if you wish to stop the new reactor along
	mn := collect.React(func(k ReactorsView) {
		func() {
			defer k.End()
		mloop:
			for {
				select {
				case <-k.Errors():
					ws.Done()
				case <-k.Closed():
					break mloop
				case <-k.Signal():
					ws.Done()
				}
			}
		}()
	})

	/*we can emit/send data for reaction using the in-channel returned bythe In() function*/
	collect.Send(2) //sends data into the root reactor
	mn.Send(40)     //sends data only into the current reactor
	collect.SendError(errors.New("sawdust"))

	ws.Wait()

	mn.SendClose("see-ya")
	//Reactors are closed by calling the .End() function
	collect.SendClose("sucker")
}

func TestMoreReceivers(t *testing.T) {
	ws := new(sync.WaitGroup)

	//master is the root reactor
	master := ReactIdentity()

	ws.Add(4)

	sl1 := Reactive(func(self ReactorsView) {
		func() {
			defer self.End()
		iloop:
			for {
				select {
				case <-self.Closed():
					break iloop
				case <-self.Errors():
				case <-self.Signal():
					ws.Done()
				}
			}
		}()
	})

	sl2 := Reactive(func(self ReactorsView) {
		func() {
			defer self.End()
		iloop:
			for {
				select {
				case <-self.Closed():
					break iloop
				case <-self.Errors():
				case <-self.Signal():
					ws.Done()
				}
			}
		}()
	})

	_ = DistributeSignals(master, sl1, sl2)

	master.Send(3)
	master.Send(40)

	// col2.End()
	// col.End()

	// master.Send(50)

	ws.Wait()
	master.SendClose("200")
}
