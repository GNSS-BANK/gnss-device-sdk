package tx

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

func TestTransmitterWriteStreamsCommandInput(t *testing.T) {
	transmitter := New(WithBinary(" custom-hackrf-transfer "))
	wantErrorOutput := &bytes.Buffer{}
	WithStderr(wantErrorOutput)(transmitter)

	var gotBinary string
	var gotArgs []string
	var gotPayload []byte
	var gotStderr io.Writer
	transmitter.run = func(
		_ context.Context,
		binary string,
		args []string,
		stdin io.Reader,
		_ io.Writer,
		stderr io.Writer,
	) error {
		gotBinary = binary
		gotArgs = append([]string(nil), args...)
		gotStderr = stderr
		var err error
		gotPayload, err = io.ReadAll(stdin)
		return err
	}

	err := transmitter.Write(
		context.Background(),
		bytes.NewReader([]byte{1, 2, 3, 4}),
		device.TXConfig{
			StreamConfig: device.StreamConfig{
				CenterFrequencyHz: 1_575_420_000,
				SampleRateHz:      10_000_000,
			},
		},
	)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if gotBinary != "custom-hackrf-transfer" {
		t.Fatalf("binary = %q, want custom-hackrf-transfer", gotBinary)
	}
	if !reflect.DeepEqual(gotArgs[:2], []string{"-t", "-"}) {
		t.Fatalf("args = %q, want TX stdin mode", gotArgs)
	}
	if gotStderr != wantErrorOutput {
		t.Fatal("stderr writer was not forwarded")
	}
	if !bytes.Equal(gotPayload, []byte{1, 2, 3, 4}) {
		t.Fatalf("payload = %v", gotPayload)
	}
	if transmitter.SampleFormat() != device.SampleFormatComplexInt8 {
		t.Fatalf("SampleFormat() = %q", transmitter.SampleFormat())
	}
}

func TestTransmitterWriteValidatesSource(t *testing.T) {
	err := New().Write(context.Background(), nil, device.TXConfig{})
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("Write() error = %v, want source error", err)
	}
}

func TestTransmitterWriteWrapsRunnerError(t *testing.T) {
	transmitter := New()
	transmitter.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return errors.New("USB disconnected")
	}

	err := transmitter.Write(context.Background(), bytes.NewReader(nil), device.TXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "write to HackRF: USB disconnected") {
		t.Fatalf("Write() error = %v", err)
	}
}
