// logger
package fwrite

import (
	"io"
	"os"
	"sync/atomic"
)

var (
	outVal atomic.Value
)

type logger struct {
	o io.Writer
}

func (w logger) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		return w.o.Write(p)
	}
	return 0, nil
}

func init() {
	setOutput(os.Stdout)
}

func setOutput(output io.Writer) {
	if out, ok := output.(*logger); ok {
		outVal.Store(out)
	} else {
		outVal.Store(&logger{output})
	}
}

func output() io.Writer {
	if w, ok := outVal.Load().(io.Writer); ok {
		return w
	} else {
		return os.Stdout
	}
}

func SetOutput(output io.Writer) {
	setOutput(output)
}
