package iio

import (
	"reflect"
	"strings"
	"testing"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

func TestRXPlan(t *testing.T) {
	commands, err := RXPlan(device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          " ip:192.168.2.1 ",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      3_000_000,
			BandwidthHz:       2_000_000,
			SampleCount:       1_024,
		},
		Gains: []device.Gain{{Stage: "hardware", ValueDB: 10}},
	}, Binaries{Attr: "attr", Read: "read"}, 4_096)
	if err != nil {
		t.Fatalf("RXPlan() error = %v", err)
	}

	want := []Command{
		{Binary: "attr", Args: []string{"-u", "ip:192.168.2.1", "-i", "-c", "ad9361-phy", "voltage0", "sampling_frequency", "3000000"}},
		{Binary: "attr", Args: []string{"-u", "ip:192.168.2.1", "-i", "-c", "ad9361-phy", "voltage0", "rf_bandwidth", "2000000"}},
		{Binary: "attr", Args: []string{"-u", "ip:192.168.2.1", "-o", "-c", "ad9361-phy", "altvoltage0", "frequency", "1575420000"}},
		{Binary: "attr", Args: []string{"-u", "ip:192.168.2.1", "-i", "-c", "ad9361-phy", "voltage0", "gain_control_mode", "manual"}},
		{Binary: "attr", Args: []string{"-u", "ip:192.168.2.1", "-i", "-c", "ad9361-phy", "voltage0", "hardwaregain", "10"}},
		{Binary: "read", Args: []string{"-u", "ip:192.168.2.1", "-b", "4096", "-s", "1024", "cf-ad9361-lpc", "voltage0", "voltage1"}},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("RXPlan() = %#v, want %#v", commands, want)
	}
}

func TestTXPlan(t *testing.T) {
	commands, err := TXPlan(device.TXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "usb:3.8.5",
			CenterFrequencyHz: 1_227_600_000,
			SampleRateHz:      4_000_000,
			BandwidthHz:       3_000_000,
		},
		Gains: []device.Gain{{Stage: "HARDWARE", ValueDB: -10.5}},
	}, Binaries{Attr: "attr", Write: "write"}, 8_192)
	if err != nil {
		t.Fatalf("TXPlan() error = %v", err)
	}

	want := []Command{
		{Binary: "attr", Args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "voltage0", "sampling_frequency", "4000000"}},
		{Binary: "attr", Args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "voltage0", "rf_bandwidth", "3000000"}},
		{Binary: "attr", Args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "altvoltage1", "frequency", "1227600000"}},
		{Binary: "attr", Args: []string{"-u", "usb:3.8.5", "-o", "-c", "ad9361-phy", "voltage0", "hardwaregain", "-10.5"}},
		{Binary: "write", Args: []string{"-u", "usb:3.8.5", "-b", "8192", "cf-ad9361-dds-core-lpc", "voltage0", "voltage1"}},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("TXPlan() = %#v, want %#v", commands, want)
	}
}

func TestRXPlanRejectsInvalidConfig(t *testing.T) {
	valid := device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "ip:192.168.2.1",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      3_000_000,
	}}

	tests := []struct {
		name       string
		bufferSize uint32
		mutate     func(*device.RXConfig)
		message    string
	}{
		{name: "URI", bufferSize: 256, mutate: func(config *device.RXConfig) { config.DeviceID = " " }, message: "URI"},
		{name: "frequency", bufferSize: 256, mutate: func(config *device.RXConfig) { config.CenterFrequencyHz = 100 }, message: "center frequency"},
		{name: "sample rate", bufferSize: 256, mutate: func(config *device.RXConfig) { config.SampleRateHz = 1_000_000 }, message: "sample rate"},
		{name: "bandwidth", bufferSize: 256, mutate: func(config *device.RXConfig) { config.BandwidthHz = 56_000_001 }, message: "RX bandwidth"},
		{name: "sample count", bufferSize: 256, mutate: func(config *device.RXConfig) { config.SampleCount = maxSampleCount }, message: "sample count"},
		{name: "buffer", bufferSize: 0, mutate: func(*device.RXConfig) {}, message: "buffer size"},
		{name: "RF amplifier", bufferSize: 256, mutate: func(config *device.RXConfig) { config.RFAmplifierEnabled = true }, message: "RF amplifier"},
		{name: "antenna power", bufferSize: 256, mutate: func(config *device.RXConfig) { config.AntennaPowerEnabled = true }, message: "antenna power"},
		{name: "trigger", bufferSize: 256, mutate: func(config *device.RXConfig) { config.HardwareTrigger = true }, message: "hardware trigger"},
		{name: "gain stage", bufferSize: 256, mutate: func(config *device.RXConfig) { config.Gains = []device.Gain{{Stage: "LNA", ValueDB: 1}} }, message: "unsupported gain stage"},
		{name: "gain range", bufferSize: 256, mutate: func(config *device.RXConfig) { config.Gains = []device.Gain{{Stage: "HARDWARE", ValueDB: 72}} }, message: "between"},
		{name: "gain step", bufferSize: 256, mutate: func(config *device.RXConfig) { config.Gains = []device.Gain{{Stage: "HARDWARE", ValueDB: 1.5}} }, message: "1 dB step"},
		{name: "duplicate gain", bufferSize: 256, mutate: func(config *device.RXConfig) {
			config.Gains = []device.Gain{{Stage: "HARDWARE", ValueDB: 1}, {Stage: "hardware", ValueDB: 2}}
		}, message: "only once"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.mutate(&config)
			_, err := RXPlan(config, Binaries{}, test.bufferSize)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("RXPlan() error = %v, want message containing %q", err, test.message)
			}
		})
	}
}

func TestTXPlanRejectsInvalidBandwidthAndGainStep(t *testing.T) {
	config := device.TXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "ip:192.168.2.1",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      3_000_000,
		BandwidthHz:       40_000_001,
	}}
	_, err := TXPlan(config, Binaries{}, 256)
	if err == nil || !strings.Contains(err.Error(), "TX bandwidth") {
		t.Fatalf("TXPlan() error = %v, want TX bandwidth error", err)
	}

	config.BandwidthHz = 0
	config.Gains = []device.Gain{{Stage: "HARDWARE", ValueDB: -10.1}}
	_, err = TXPlan(config, Binaries{}, 256)
	if err == nil || !strings.Contains(err.Error(), "0.25 dB step") {
		t.Fatalf("TXPlan() error = %v, want TX gain step error", err)
	}
}

func TestPlansRejectMissingBinariesBeforeExecution(t *testing.T) {
	rxConfig := device.RXConfig{StreamConfig: device.StreamConfig{
		DeviceID:          "ip:192.168.2.1",
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      3_000_000,
	}}
	_, err := RXPlan(rxConfig, Binaries{Read: "iio_readdev"}, 256)
	if err == nil || !strings.Contains(err.Error(), "iio_attr binary") {
		t.Fatalf("RXPlan() error = %v, want iio_attr binary error", err)
	}

	txConfig := device.TXConfig{StreamConfig: rxConfig.StreamConfig}
	_, err = TXPlan(txConfig, Binaries{Attr: "iio_attr"}, 256)
	if err == nil || !strings.Contains(err.Error(), "iio_writedev binary") {
		t.Fatalf("TXPlan() error = %v, want iio_writedev binary error", err)
	}
}
