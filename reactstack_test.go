package flux

import (
	"errors"
	"log"
	"sync"
	"testing"
)

func TestChannelConnect(t *testing.T) {
	stream := SignalCollector()
	feed := Reactive(ChannelReactProcessor(stream))

	feed.Send(20)

	feed.SendClose(404)

	do := <-stream.Signals()

	log.Printf("do:", do)

	if do != 20 {
		t.Fatalf("Inccorect value received %d expected %d", do, 20)
	}

	feed.Destroy()
}

func TestConnect(t *testing.T) {

	ws := new(sync.WaitGroup)

	//ReactIdentity returns reactor that simply pipes what input it gets to its output if it has a connection

	//collect is the root reactor
	collect := ReactIdentity()

	ws.Add(2)

	mn := Reactive(func(k ReactorsView) {
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

	ok := collect.Bind(mn)

	if !ok {
		t.Fatal("Unable to create binding")
	}

	collect.Send(2)
	collect.SendError(errors.New("sawdust"))

	ws.Wait()

	collect.SendClose("sucker")
}

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

func TestDistributor(t *testing.T) {
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

	ws.Wait()
	master.SendClose("200")
}

func TestMergers(t *testing.T) {
	ws := new(sync.WaitGroup)

	ws.Add(2)

	mo := ReactIdentity()
	mp := ReactIdentity()

	merged := MergeReactors(mo, mp)

	merged.React(func(v ReactorsView) {
		defer v.End()
	mop:
		for {
			select {
			case <-v.Errors():
			case <-v.Closed():
				// ws.Done()
				break mop
			case <-v.Signal():
				ws.Done()
			}
		}
	})

	mo.Send(3)
	mp.Send(40)

	ws.Wait()

	mo.SendClose("200")
	mp.SendClose("600")

}

func TestLifters(t *testing.T) {
	ws := new(sync.WaitGroup)

	ws.Add(2)

	mo := DataReact(func(d Signal) Signal {
		vk, _ := d.(int)
		return vk * 100
	})

	mo.React(DataReactProcessor(func(data Signal) Signal {
		vk, _ := data.(int)
		return vk / 4
	}))

	mp := DataReact(func(d Signal) Signal {
		vk, _ := d.(int)
		return vk * 20
	})

	liftd, err := LiftReactors(mo, mp)

	if err != nil {
		t.Fatal(err)
	}

	liftd.React(DataReactProcessor(func(data Signal) Signal {
		ws.Done()
		return data
	}))

	mo.Send(3)
	mo.Send(40)

	ws.Wait()

	mo.SendClose("200")
	mp.SendClose("600")

}
