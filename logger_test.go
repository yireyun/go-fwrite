package fwrite

import (
	"testing"
)

func TestLogWrite(t *testing.T) {
	output().Write([]byte("Test"))
}
