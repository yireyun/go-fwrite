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

func init() {
	outVal.Store(os.Stdout)
}

func SetOutput(out io.Writer) {
	outVal.Store(out)
}

func output() io.Writer {
	if w, ok := outVal.Load().(io.Writer); ok {
		return w
	} else {
		return os.Stdout
	}

}
