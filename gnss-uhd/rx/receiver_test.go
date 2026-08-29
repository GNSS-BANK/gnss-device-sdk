package rx

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

func TestReceiverReadForwardsConfigAndSettings(t *testing.T) {
	receiver := New(WithChannel(1), WithBufferLength(20), WithChunkSize(512), WithRXBufferSize(4_096), WithAutomaticGain(false))
	receiver.read = func(_ context.Context, dst io.Writer, config device.RXConfig, settings stream.RXSettings) error {
		if config.DeviceID != "serial=1234" || settings.Channel != 1 || settings.BufferLength != 20 || settings.ChunkSamples != 512 {
			t.Fatalf("unexpected config/settings: %#v %#v", config, settings)
		}
		_, err := dst.Write([]byte{1, 2, 3, 4})
		return err
	}

	var output bytes.Buffer
	err := receiver.Read(context.Background(), &output, device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          " serial=1234 ",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      10_000_000,
	}})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if output.Len() != 4 || receiver.SampleFormat() != device.SampleFormatComplexInt16LE {
		t.Fatalf("output=%d format=%q", output.Len(), receiver.SampleFormat())
	}
}

func TestReceiverReadValidatesAndWrapsErrors(t *testing.T) {
	if err := New().Read(context.Background(), nil, device.RXConfig{}); err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("nil destination error = %v", err)
	}
	receiver := New()
	receiver.read = func(context.Context, io.Writer, device.RXConfig, stream.RXSettings) error {
		return errors.New("overflow")
	}
	err := receiver.Read(context.Background(), io.Discard, device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID: "addr=1", CenterFrequencyHz: 1, SampleRateHz: 1,
	}})
	if err == nil || !strings.Contains(err.Error(), "read from UHD: overflow") {
		t.Fatalf("Read() error = %v", err)
	}
}
