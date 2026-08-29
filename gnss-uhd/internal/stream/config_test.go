package stream

import (
	"strings"
	"testing"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

func TestValidateRXAcceptsUHDSettings(t *testing.T) {
	automaticGain := false
	err := ValidateRX(device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "type=x300,addr=192.168.10.2",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      200_000_000,
			SampleCount:       1_000_000,
		},
		Gains: []device.Gain{{Stage: "PGA", ValueDB: 20}},
	}, RXSettings{
		Channel:       0,
		BufferLength:  10,
		ChunkSamples:  256,
		BufferBytes:   2_048,
		AutomaticGain: &automaticGain,
	})
	if err != nil {
		t.Fatalf("ValidateRX() error = %v", err)
	}
}

func TestValidateRXRejectsInvalidConfig(t *testing.T) {
	valid := device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "addr=192.168.10.2",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      10_000_000,
	}}
	settings := RXSettings{BufferLength: 10, ChunkSamples: 256, BufferBytes: 2_048}

	tests := []struct {
		name    string
		mutate  func(*device.RXConfig, *RXSettings)
		message string
	}{
		{name: "device", mutate: func(config *device.RXConfig, _ *RXSettings) { config.DeviceID = "" }, message: "device args"},
		{name: "frequency", mutate: func(config *device.RXConfig, _ *RXSettings) { config.CenterFrequencyHz = 0 }, message: "center frequency"},
		{name: "sample rate", mutate: func(config *device.RXConfig, _ *RXSettings) { config.SampleRateHz = 0 }, message: "sample rate"},
		{name: "bandwidth", mutate: func(config *device.RXConfig, _ *RXSettings) { config.BandwidthHz = 1 }, message: "bandwidth"},
		{name: "channel", mutate: func(_ *device.RXConfig, settings *RXSettings) { settings.Channel = -1 }, message: "channel"},
		{name: "chunk", mutate: func(_ *device.RXConfig, settings *RXSettings) { settings.ChunkSamples = 0 }, message: "chunk"},
		{name: "buffer", mutate: func(_ *device.RXConfig, settings *RXSettings) { settings.BufferBytes = 1_000 }, message: "at least two chunks"},
		{name: "gain stage", mutate: func(config *device.RXConfig, _ *RXSettings) { config.Gains = []device.Gain{{Stage: " ", ValueDB: 1}} }, message: "gain stage"},
		{name: "duplicate gain", mutate: func(config *device.RXConfig, _ *RXSettings) {
			config.Gains = []device.Gain{{Stage: "PGA", ValueDB: 1}, {Stage: "pga", ValueDB: 2}}
		}, message: "more than once"},
		{name: "automatic gain", mutate: func(config *device.RXConfig, settings *RXSettings) {
			enabled := true
			settings.AutomaticGain = &enabled
			config.Gains = []device.Gain{{Stage: "PGA", ValueDB: 1}}
		}, message: "automatic gain"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			currentSettings := settings
			test.mutate(&config, &currentSettings)
			err := ValidateRX(config, currentSettings)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("ValidateRX() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestValidateTXRejectsUnsupportedCommonFlags(t *testing.T) {
	config := device.TXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "serial=1234",
		CenterFrequencyHz: 1_227_600_000,
		SampleRateHz:      10_000_000,
	}, HardwareTrigger: true}
	err := ValidateTX(config, TXSettings{BufferLength: 10, ChunkSamples: 256})
	if err == nil || !strings.Contains(err.Error(), "hardware trigger") {
		t.Fatalf("ValidateTX() error = %v", err)
	}
}
