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

type invocation struct {
	binary string
	args   []string
}

func TestReceiverReadConfiguresAndStreams(t *testing.T) {
	diagnostics := &bytes.Buffer{}
	receiver := New(
		WithIIOAttrBinary(" custom-attr "),
		WithIIOReaddevBinary(" custom-read "),
		WithBufferSize(4_096),
		WithStderr(diagnostics),
	)

	var calls []invocation
	receiver.run = func(
		_ context.Context,
		binary string,
		args []string,
		stdin io.Reader,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		if stdin != nil {
			t.Fatal("RX command received unexpected stdin")
		}
		if stderr != diagnostics {
			t.Fatal("stderr writer was not forwarded")
		}
		calls = append(calls, invocation{binary: binary, args: append([]string(nil), args...)})
		if binary == "custom-read" {
			_, err := stdout.Write([]byte{1, 2, 3, 4})
			return err
		}
		if stdout != io.Discard {
			t.Fatal("iio_attr stdout must be discarded")
		}
		return nil
	}

	var destination bytes.Buffer
	err := receiver.Read(context.Background(), &destination, device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "ip:192.168.2.1",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      3_000_000,
		},
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	want := []invocation{
		{binary: "custom-attr", args: []string{"-u", "ip:192.168.2.1", "-i", "-c", "ad9361-phy", "voltage0", "sampling_frequency", "3000000"}},
		{binary: "custom-attr", args: []string{"-u", "ip:192.168.2.1", "-o", "-c", "ad9361-phy", "altvoltage0", "frequency", "1575420000"}},
		{binary: "custom-read", args: []string{"-u", "ip:192.168.2.1", "-b", "4096", "cf-ad9361-lpc", "voltage0", "voltage1"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	if !bytes.Equal(destination.Bytes(), []byte{1, 2, 3, 4}) {
		t.Fatalf("destination = %v", destination.Bytes())
	}
	if receiver.SampleFormat() != device.SampleFormatComplexInt16LE {
		t.Fatalf("SampleFormat() = %q", receiver.SampleFormat())
	}
}

func TestReceiverReadValidatesDestination(t *testing.T) {
	err := New().Read(context.Background(), nil, device.RXConfig{})
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("Read() error = %v, want destination error", err)
	}
}

func TestReceiverReadWrapsConfigurationError(t *testing.T) {
	receiver := New()
	receiver.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return errors.New("Pluto disconnected")
	}
	err := receiver.Read(context.Background(), io.Discard, validRXConfig())
	if err == nil || !strings.Contains(err.Error(), "configure Pluto RX: Pluto disconnected") {
		t.Fatalf("Read() error = %v", err)
	}
}

func TestReceiverReadWrapsStreamingError(t *testing.T) {
	receiver := New()
	call := 0
	receiver.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		call++
		if call == 3 {
			return errors.New("USB transfer failed")
		}
		return nil
	}
	err := receiver.Read(context.Background(), io.Discard, validRXConfig())
	if err == nil || !strings.Contains(err.Error(), "read from Pluto: USB transfer failed") {
		t.Fatalf("Read() error = %v", err)
	}
}

func validRXConfig() device.RXConfig {
	return device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "ip:192.168.2.1",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      3_000_000,
	}}
}
