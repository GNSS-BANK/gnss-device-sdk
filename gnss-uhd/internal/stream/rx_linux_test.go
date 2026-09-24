//go:build linux && cgo

package stream

import (
	"errors"
	"io"
	"testing"

	"hz.tools/sdr/uhd"
)

func TestIsRecoverableReadError(t *testing.T) {
	recoverable := []error{
		uhd.ErrRxMetadataTimeout,
		uhd.ErrRxMetadataOverflow,
		uhd.ErrIO,
		uhd.ErrUSB,
		uhd.ErrOS,
		uhd.ErrRuntime,
		io.EOF,
	}
	for _, err := range recoverable {
		if !isRecoverableReadError(err) {
			t.Fatalf("expected %v to be recoverable", err)
		}
	}

	if isRecoverableReadError(errors.New("fatal read error")) {
		t.Fatal("unexpected recoverable classification for an unknown error")
	}
}
