package tx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-uhd/internal/stream"
)

func TestTransmitterWriteForwardsConfigAndSettings(t *testing.T) {
	transmitter := New(WithChannel(2), WithBufferLength(20), WithChunkSize(512))
	transmitter.write = func(_ context.Context, src io.Reader, config device.TXConfig, settings stream.TXSettings) error {
		if config.DeviceID != "type=x310" || settings.Channel != 2 || settings.BufferLength != 20 || settings.ChunkSamples != 512 {
			t.Fatalf("unexpected config/settings: %#v %#v", config, settings)
		}
		_, err := io.ReadAll(src)
		return err
	}
	err := transmitter.Write(context.Background(), bytes.NewReader([]byte{1, 2, 3, 4}), device.TXConfig{StreamConfig: device.StreamConfig{
		DeviceID: " type=x310 ", CenterFrequencyHz: 1, SampleRateHz: 1,
	}})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if transmitter.SampleFormat() != device.SampleFormatComplexInt16LE {
		t.Fatalf("SampleFormat() = %q", transmitter.SampleFormat())
	}
}

func TestTransmitterWriteValidatesAndWrapsErrors(t *testing.T) {
	if err := New().Write(context.Background(), nil, device.TXConfig{}); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("nil source error = %v", err)
	}
	transmitter := New()
	transmitter.write = func(context.Context, io.Reader, device.TXConfig, stream.TXSettings) error {
		return errors.New("underrun")
	}
	err := transmitter.Write(context.Background(), bytes.NewReader(nil), device.TXConfig{StreamConfig: device.StreamConfig{
		DeviceID: "addr=1", CenterFrequencyHz: 1, SampleRateHz: 1,
	}})
	if err == nil || !strings.Contains(err.Error(), "write to UHD: underrun") {
		t.Fatalf("Write() error = %v", err)
	}
}
