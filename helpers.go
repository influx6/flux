package flux

import (
	"runtime"
	"runtime/debug"
	"strings"
)

//Report provides a nice abstaction for doing basic report
func Report(e error, msg string) {
	if e != nil {
		log.Error("Message: (%s) with Error: (%+v)", msg, e)
	} else {
		log.Info("Message: (%s) with NoError", msg)
	}
}

//GoDefer letsw you run a function inside a goroutine that gets a defer recovery
func GoDefer(title string, fx func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				var stacks []byte
				runtime.Stack(stacks, true)
				log.Debug("---------%s-Panic----------------:", strings.ToUpper(title))
				log.Debug("Stack Error: %+s", err)
				log.Debug("Debug Stack: %+s", debug.Stack())
				log.Debug("Stack List: %+s", stacks)
				log.Debug("---------%s--END-----------------:", strings.ToUpper(title))
			}
		}()
		fx()
	}()
}

//Close provides a basic io.WriteCloser write method
func (w *FuncWriter) Close() error {
	w.fx = nil
	return nil
}

//Write provides a basic io.Writer write method
func (w *FuncWriter) Write(b []byte) (int, error) {
	w.fx(b)
	return len(b), nil
}

//NewFuncWriter returns a new function writer instance
func NewFuncWriter(fx func([]byte)) *FuncWriter {
	return &FuncWriter{fx}
}

type (
	//FuncWriter provides a means of creation io.Writer on functions
	FuncWriter struct {
		fx func([]byte)
	}
)
