package flux

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestMousePosition provides a example test of a reactor that process mouse position (a slice of two values)
func TestMousePosition(t *testing.T) {
	var count int64
	var ws sync.WaitGroup

	ws.Add(2)

	pos := Reactive(func(r Reactor, err error, data interface{}) {
		if err != nil {
			r.ReplyError(err)
			return
		}

		atomic.AddInt64(&count, 1)
		r.Reply(data)
	})

	go func() {
		//add about 3000 mouse events
		for i := 0; i < 2000; i++ {
			pos.Send([]int{i * 3, i + 1})
		}
		ws.Done()
	}()

	go func() {
		//add about 3000 mouse events
		for i := 0; i < 1000; i++ {
			pos.Send([]int{i * 3, i + 1})
		}
		ws.Done()
	}()

	ws.Wait()
	LogPassed(t, "Delivered all 3000 events")

	pos.Close()

	if atomic.LoadInt64(&count) != 3000 {
		FatalFailed(t, "Total processed values is not equal, expected %d but got %d", 3000, count)
	}

	LogPassed(t, "Total mouse data was processed with count %d", count)
}

// TestPartyOf2 test two independent reactors binding
func TestPartyOf2(t *testing.T) {
	var ws sync.WaitGroup
	ws.Add(2)

	dude := Reactive(func(r Reactor, err error, data interface{}) {
		r.Reply(data)
		ws.Done()
	})

	dudette := Reactive(func(r Reactor, err error, data interface{}) {
		r.Reply("4000")
		ws.Done()
	})

	dude.Bind(dudette, true)

	dude.Send("3000")

	// dudette.Close()
	ws.Wait()

	dude.Close()
}

func TestMerge(t *testing.T) {
	var ws sync.WaitGroup
	ws.Add(2)

	mo := ReactIdentity()
	mp := ReactIdentity()

	me := MergeReactors(mo, mp)

	me.React(func(v Reactor, err error, data interface{}) {
		ws.Done()
	}, true)

	mo.Send(1)
	mp.Send(2)

	me.Close()

	//merge will not react to this
	mp.Send(4)

	ws.Wait()

	mo.Close()
	mp.Close()
}

func TestDistribute(t *testing.T) {
	var ws sync.WaitGroup
	ws.Add(300)

	master := ReactIdentity()

	slave := Reactive(func(r Reactor, err error, data interface{}) {
		if _, ok := data.(int); !ok {
			FatalFailed(t, "Data %+v is not int type", data)
		}
		ws.Done()
	})

	slave2 := Reactive(func(r Reactor, err error, data interface{}) {
		if _, ok := data.(int); !ok {
			FatalFailed(t, "Data %+v is not int type", data)
		}
		ws.Done()
	})

	DistributeSignals(master, slave, slave2)

	for i := 0; i < 150; i++ {
		master.Send(i)
	}
	LogPassed(t, "Successfully Sent 150 numbers")

	ws.Wait()

	LogPassed(t, "Successfully Processed 150 numbers")
	master.Close()
	slave.Close()
	slave2.Close()
}

func TestLift(t *testing.T) {
	var ws sync.WaitGroup
	ws.Add(2)

	master := ReactIdentity()

	slave := Reactive(func(r Reactor, err error, data interface{}) {
		if data != 40 {
			FatalFailed(t, "Incorrect value recieved,expect %d got %d", 40, data)
		}
		r.Reply(data.(int) * 20)
		ws.Done()
	})

	slave.React(func(r Reactor, err error, data interface{}) {
		if data != 800 {
			FatalFailed(t, "Incorrect value recieved,expect %d got %d", 800, data)
		}
		ws.Done()
	}, true)

	Lift(true, master, slave)

	master.Send(40)

	ws.Wait()

	LogPassed(t, "Successfully Lifted numbers between 2 Reactors")
	master.Close()
	slave.Close()
}
