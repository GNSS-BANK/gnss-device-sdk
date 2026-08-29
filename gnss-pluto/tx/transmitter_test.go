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

type invocation struct {
	binary string
	args   []string
}

func TestTransmitterWriteConfiguresAndStreams(t *testing.T) {
	diagnostics := &bytes.Buffer{}
	transmitter := New(
		WithIIOAttrBinary(" custom-attr "),
		WithIIOWritedevBinary(" custom-write "),
		WithBufferSize(8_192),
		WithStderr(diagnostics),
	)

	var calls []invocation
	var payload []byte
	transmitter.run = func(
		_ context.Context,
		binary string,
		args []string,
		stdin io.Reader,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		if stdout != io.Discard {
			t.Fatal("TX command stdout must be discarded")
		}
		if stderr != diagnostics {
			t.Fatal("stderr writer was not forwarded")
		}
		calls = append(calls, invocation{binary: binary, args: append([]string(nil), args...)})
		if binary == "custom-write" {
			var err error
			payload, err = io.ReadAll(stdin)
			return err
		}
		if stdin != nil {
			t.Fatal("iio_attr received unexpected stdin")
		}
		return nil
	}

	err := transmitter.Write(context.Background(), bytes.NewReader([]byte{1, 2, 3, 4}), device.TXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "usb:3.8.5",
			CenterFrequencyHz: 1_227_600_000,
			SampleRateHz:      4_000_000,
			SampleCount:       1,
		},
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	want := []invocation{
		{binary: "custom-attr", args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "voltage0", "sampling_frequency", "4000000"}},
		{binary: "custom-attr", args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "altvoltage1", "frequency", "1227600000"}},
		{binary: "custom-write", args: []string{"-u", "usb:3.8.5", "-b", "8192", "-s", "1", "cf-ad9361-dds-core-lpc", "voltage0", "voltage1"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	if !bytes.Equal(payload, []byte{1, 2, 3, 4}) {
		t.Fatalf("payload = %v", payload)
	}
	if transmitter.SampleFormat() != device.SampleFormatComplexInt16LE {
		t.Fatalf("SampleFormat() = %q", transmitter.SampleFormat())
	}
}

func TestTransmitterWriteValidatesSource(t *testing.T) {
	err := New().Write(context.Background(), nil, device.TXConfig{})
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("Write() error = %v, want source error", err)
	}
}

func TestTransmitterWriteWrapsConfigurationError(t *testing.T) {
	transmitter := New()
	transmitter.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return errors.New("Pluto disconnected")
	}
	err := transmitter.Write(context.Background(), bytes.NewReader(nil), validTXConfig())
	if err == nil || !strings.Contains(err.Error(), "configure Pluto TX: Pluto disconnected") {
		t.Fatalf("Write() error = %v", err)
	}
}

func TestTransmitterWriteWrapsStreamingError(t *testing.T) {
	transmitter := New()
	call := 0
	transmitter.run = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		call++
		if call == 3 {
			return errors.New("USB transfer failed")
		}
		return nil
	}
	err := transmitter.Write(context.Background(), bytes.NewReader(nil), validTXConfig())
	if err == nil || !strings.Contains(err.Error(), "write to Pluto: USB transfer failed") {
		t.Fatalf("Write() error = %v", err)
	}
}

func validTXConfig() device.TXConfig {
	return device.TXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "ip:192.168.2.1",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      3_000_000,
	}}
}
