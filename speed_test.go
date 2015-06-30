package flux

import (
	"bytes"
	"fmt"
	"log"
	"testing"
	"time"
)

const (
	maxlistener = 4
	maxcount    = 100
)

func TestBaseStreamSpeed(t *testing.T) {
	var buff bytes.Buffer
	sm := NewBaseStream()

	if sm == nil {
		t.Fatal("Unable to  create streamer")
	}

	sm.Subscribe(func(b interface{}, sub *Sub) {
		//do something
		buff.WriteString(fmt.Sprintf("%+s", b))
		time.Sleep(time.Duration(1) * time.Nanosecond)
	})

	now := time.Now()
	counter := maxcount

	for {
		if counter <= 0 {
			break
		}

		sm.Emit(counter)
		counter--
	}

	end := time.Now()

	log.Printf("BaseStream Start Time: %+s", now)
	log.Printf("BaseStream End Time: %+s", now)
	elapse := end.Sub(now)
	log.Printf("BaseStream Speed: %+s ", elapse)
	buff.Reset()
}

func TestStackSpeed(t *testing.T) {

	var buff bytes.Buffer
	sm := NewStack(func(data interface{}, _ Stacks) interface{} {
		return data
	}, nil)

	sm.Stack(func(b interface{}, s Stacks) interface{} {
		buff.WriteString(fmt.Sprintf("%+s", b))
		time.Sleep(time.Duration(1) * time.Nanosecond)
		return b
	})

	now := time.Now()
	counter := maxcount

	for {
		if counter <= 0 {
			break
		}

		sm.Emit(counter)
		counter--
	}

	end := time.Now()

	log.Printf("StackStream Start Time: %+s", now)
	log.Printf("StackStream End Time: %+s", now)
	elapse := end.Sub(now)
	log.Printf("StackStream Speed: %+s ", elapse)
	buff.Reset()
}
