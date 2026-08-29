package rx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

func TestReceiverReadStreamsCommandOutput(t *testing.T) {
	receiver := New(WithBinary(" custom-hackrf-transfer "))
	wantErrorOutput := &bytes.Buffer{}
	WithStderr(wantErrorOutput)(receiver)

	var gotBinary string
	var gotArgs []string
	var gotStderr io.Writer
	receiver.run = func(
		_ context.Context,
		binary string,
		args []string,
		_ io.Reader,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		gotBinary = binary
		gotArgs = append([]string(nil), args...)
		gotStderr = stderr
		_, err := stdout.Write([]byte{1, 2, 3, 4})
		return err
	}

	var destination bytes.Buffer
	err := receiver.Read(context.Background(), &destination, device.RXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if gotBinary != "custom-hackrf-transfer" {
		t.Fatalf("binary = %q, want custom-hackrf-transfer", gotBinary)
	}
	if !reflect.DeepEqual(gotArgs[:2], []string{"-r", "-"}) {
		t.Fatalf("args = %q, want RX stdout mode", gotArgs)
	}
	if gotStderr != wantErrorOutput {
		t.Fatal("stderr writer was not forwarded")
	}
	if !bytes.Equal(destination.Bytes(), []byte{1, 2, 3, 4}) {
		t.Fatalf("destination = %v", destination.Bytes())
	}
	if receiver.SampleFormat() != device.SampleFormatComplexInt8 {
		t.Fatalf("SampleFormat() = %q", receiver.SampleFormat())
	}
}

func TestReceiverReadValidatesDestination(t *testing.T) {
	err := New().Read(context.Background(), nil, device.RXConfig{})
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("Read() error = %v, want destination error", err)
	}
}

func TestReceiverReadWrapsRunnerError(t *testing.T) {
	receiver := New()
	receiver.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return errors.New("USB disconnected")
	}

	err := receiver.Read(context.Background(), io.Discard, device.RXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "read from HackRF: USB disconnected") {
		t.Fatalf("Read() error = %v", err)
	}
}
