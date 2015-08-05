package flux

import (
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
	}, nil, true)

	g := sc.Stack(func(data interface{}, _ Stacks) interface{} {
		return data
	}, true)

	defer sc.Emit(7)
	defer g.Emit(82)
	// defer g.Close()

	_ = LogStack(g)

	_ = sc.Stack(func(data interface{}, _ Stacks) interface{} {
		return 20
	}, true)

	xres := sc.Emit(1)
	yres := g.Emit(30)

	if xres == yres {
		log.Fatalf("Equal unexpect values %d %d", xres, yres)
	}

	xres = sc.Emit(40)
	g.RootEmit(20)

	if xres == yres {
		log.Fatalf("Equal unexpect values %d %d", xres, yres)
	}

	sc.Close()
}
