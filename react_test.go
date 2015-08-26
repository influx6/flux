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

// TestPartyOf2 test two independent reactors binding to each other using the bind option with a small dangerous trick (can we create a feedback queue endlessly)
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

	mp.Send(4)

	ws.Wait()

	mo.Close()
	mp.Close()
}
