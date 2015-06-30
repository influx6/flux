package flux

import (
	"log"
	"testing"
)

func TestStackers(t *testing.T) {
	sc := NewStack(func(data interface{}, _ Stacks) interface{} {
		nm, ok := data.(int)
		if !ok {
			log.Fatalf("invalid type: expected int got %+s", data)
			return nil
		}

		return nm
	}, nil)

	g := sc.Stack(func(data interface{}, _ Stacks) interface{} {
		return data
	})

	_ = LogStack(g)

	_ = sc.Stack(func(data interface{}, _ Stacks) interface{} {
		return 20
	})

	xres := sc.Emit(1)
	yres := g.Emit(30)

	if xres == yres {
		log.Fatalf("Equal unexpect values %d %d", xres, yres)
	}

	g.Unstack()

	xres = sc.Emit(40)

	if xres == yres {
		log.Fatalf("Equal unexpect values %d %d", xres, yres)
	}
}
