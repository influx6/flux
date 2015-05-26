package flux

import (
	"sync"
	"testing"
)

func TestBaseStream(t *testing.T) {
	sm := NewBaseStream()

	if sm == nil {
		t.Fatal("Unable to  create streamer")
	}

	ws := new(sync.WaitGroup)

	ws.Add(2)

	sm.Subscribe(func(b interface{}, _ *Sub) {
		ws.Done()
	})

	sm.Write([]byte("God!"))
	sm.Emit("Love!")
	ws.Wait()
}

func TestByteStream(t *testing.T) {
	sm := NewByteStream()

	if sm == nil {
		t.Fatal("Unable to  create streamer")
	}

	ws := new(sync.WaitGroup)

	ws.Add(2)

	sm.Subscribe(func(b interface{}, _ *Sub) {
		ws.Done()
	})

	sm.Write([]byte("God!"))
	sm.Emit("Love!")
	ws.Wait()
}

func TestDoByteStream(t *testing.T) {
	sm := NewByteStream()

	if sm == nil {
		t.Fatal("Unable to  create streamer")
	}

	sx, err := DoByteStream(sm, func(b []byte) []byte {
		t.Logf("BeforeMod: %s", b)
		b[0] = b[0] + 1
		t.Logf("AfterMod: %s", b)
		return b
	})

	if err != nil {
		t.Fatal("Unable to  create modified streamer", err)
	}

	ws := new(sync.WaitGroup)

	ws.Add(2)

	sx.Subscribe(func(b interface{}, _ *Sub) {
		ws.Done()
	})

	sm.Write([]byte("God!"))
	sm.Emit("Love!")
	ws.Wait()
}
