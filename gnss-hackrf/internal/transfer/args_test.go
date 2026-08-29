package transfer

import (
	"strings"
	"testing"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

func TestRXArgs(t *testing.T) {
	args, err := RXArgs(device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          " 0000000001 ",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
			BandwidthHz:       8_000_000,
			SampleCount:       2_000_000,
		},
		Gains: []device.Gain{
			{Stage: "lna", ValueDB: 16},
			{Stage: "VGA", ValueDB: 22},
		},
		RFAmplifierEnabled: true,
		HardwareTrigger:    true,
	})
	if err != nil {
		t.Fatalf("RXArgs() error = %v", err)
	}

	want := []string{
		"-r", "-",
		"-d", "0000000001",
		"-f", "1575420000",
		"-s", "10000000",
		"-b", "8000000",
		"-n", "2000000",
		"-a", "1",
		"-p", "0",
		"-l", "16",
		"-g", "22",
		"-H",
	}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("RXArgs() = %q, want %q", args, want)
	}
}

func TestTXArgsUsesDefaults(t *testing.T) {
	args, err := TXArgs(device.TXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_227_600_000,
			SampleRateHz:      2_000_000,
		},
		AntennaPowerEnabled: true,
	})
	if err != nil {
		t.Fatalf("TXArgs() error = %v", err)
	}

	want := []string{
		"-t", "-",
		"-f", "1227600000",
		"-s", "2000000",
		"-a", "0",
		"-p", "1",
		"-x", "0",
	}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("TXArgs() = %q, want %q", args, want)
	}
}

func TestRXArgsRejectsInvalidConfig(t *testing.T) {
	valid := device.RXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
	}

	tests := []struct {
		name    string
		mutate  func(*device.RXConfig)
		message string
	}{
		{
			name: "frequency",
			mutate: func(config *device.RXConfig) {
				config.CenterFrequencyHz = 0
			},
			message: "center frequency",
		},
		{
			name: "sample rate",
			mutate: func(config *device.RXConfig) {
				config.SampleRateHz = 1_000_000
			},
			message: "sample rate",
		},
		{
			name: "bandwidth",
			mutate: func(config *device.RXConfig) {
				config.BandwidthHz = 4_000_000
			},
			message: "bandwidth",
		},
		{
			name: "sample count",
			mutate: func(config *device.RXConfig) {
				config.SampleCount = maxSampleCount
			},
			message: "sample count",
		},
		{
			name: "gain step",
			mutate: func(config *device.RXConfig) {
				config.Gains = []device.Gain{{Stage: "LNA", ValueDB: 10}}
			},
			message: "8 dB step",
		},
		{
			name: "duplicate gain",
			mutate: func(config *device.RXConfig) {
				config.Gains = []device.Gain{
					{Stage: "VGA", ValueDB: 20},
					{Stage: "vga", ValueDB: 22},
				}
			},
			message: "more than once",
		},
		{
			name: "unsupported gain",
			mutate: func(config *device.RXConfig) {
				config.Gains = []device.Gain{{Stage: "TX", ValueDB: 1}}
			},
			message: "unsupported gain stage",
		},
		{
			name: "fractional gain",
			mutate: func(config *device.RXConfig) {
				config.Gains = []device.Gain{{Stage: "VGA", ValueDB: 20.5}}
			},
			message: "non-negative integer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.mutate(&config)

			_, err := RXArgs(config)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("RXArgs() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}

func TestTXArgsRejectsRXGainStage(t *testing.T) {
	_, err := TXArgs(device.TXConfig{
		StreamConfig: device.StreamConfig{
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
		Gains: []device.Gain{{Stage: "LNA", ValueDB: 8}},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported gain stage") {
		t.Fatalf("TXArgs() error = %v, want unsupported gain stage", err)
	}
}
