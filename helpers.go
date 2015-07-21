package flux

import (
	"log"
	"runtime"
	"runtime/debug"
	"strings"
)

//GoDefer letsw you run a function inside a goroutine that gets a defer recovery
func GoDefer(title string, fx func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				var stacks []byte
				runtime.Stack(stacks, true)
				log.Printf("---------%s-Panic----------------:", strings.ToUpper(title))
				log.Printf("Stack Error: %+s", err)
				log.Printf("Debug Stack: %+s", debug.Stack())
				log.Printf("Stack List: %+s", stacks)
				log.Printf("---------%s--END-----------------:", strings.ToUpper(title))
			}
		}()
		fx()
	}()
}
