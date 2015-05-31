package flux

import (
	"sync"
	"testing"
	"time"
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

func TestTimedByteStream(t *testing.T) {
	sm := TimedByteStream(1, time.Duration(1)*time.Second)

	if sm == nil {
		t.Fatal("Unable to  create streamer")
	}

	ws := new(sync.WaitGroup)

	ws.Add(2)

	sm.Subscribe(func(b interface{}, _ *Sub) {
		t.Logf("Subscription Recieved: %s : %+v", b, b)
		ws.Done()
	})

	sm.Write([]byte("God!"))
	sm.Emit("Love!")
	ws.Wait()
	sm.Close()
}

func TestRecordStream(t *testing.T) {
	rs := NewRecordedStream()

	if rs == nil {
		t.Fatal("unable to create recordedstream")
	}

	defer rs.Close()

	rs.Subscribe(func(data interface{}, _ *Sub) {
		_, ok := data.([]byte)
		if !ok {
			t.Fatal("Data received is not a byte splice:", data)
		}
	})

	rs.Write([]byte("Wonder"))
	rs.Write([]byte("ful"))
	rs.Write([]byte("!!"))
}

func TestStreamRecord(t *testing.T) {
	rs := NewRecordedStream()

	if rs == nil {
		t.Fatal("unable to create recordedstream")
	}

	defer rs.Close()

	rs.Write([]byte("Wonder"))
	rs.Write([]byte("ful"))
	rs.Write([]byte("!!"))

	ds, err := rs.Stream()

	if err != nil {
		t.Fatal("Unable to create stream from source:", err)
	}

	ds.Subscribe(func(data interface{}, _ *Sub) {
		_, ok := data.([]byte)
		if !ok {
			t.Fatal("Data received is not a byte splice:", data)
		}
	})
	ds.Push()

}
