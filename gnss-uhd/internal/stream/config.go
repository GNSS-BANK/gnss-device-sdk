package stream

import (
	"errors"
	"fmt"
	"math"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

// ValidateRX проверяет общие и специфичные для UHD настройки приёма.
func ValidateRX(config device.RXConfig, settings RXSettings) error {
	if err := validateCommon(config.StreamConfig, config.RFAmplifierEnabled, config.AntennaPowerEnabled, config.HardwareTrigger); err != nil {
		return err
	}
	switch {
	case settings.Channel < 0:
		return errors.New("UHD RX channel must be >= 0")
	case settings.BufferLength <= 0:
		return errors.New("UHD buffer length must be > 0")
	case settings.ChunkSamples <= 0:
		return errors.New("UHD chunk size must be > 0")
	case settings.BufferBytes <= 0:
		return errors.New("UHD RX buffer size must be > 0")
	case settings.SettleDelay < 0:
		return errors.New("UHD settle delay must be >= 0")
	case settings.RestartDelay < 0:
		return errors.New("UHD restart delay must be >= 0")
	case settings.AutomaticGain != nil && *settings.AutomaticGain && len(config.Gains) > 0:
		return errors.New("UHD RX gains cannot be used when automatic gain is enabled")
	}
	if settings.ChunkSamples > int(^uint(0)>>1)/BytesPerSample {
		return errors.New("UHD chunk size is too large")
	}
	chunkBytes := settings.ChunkSamples * BytesPerSample
	if settings.BufferBytes/chunkBytes < 2 {
		return fmt.Errorf("UHD RX buffer must fit at least two chunks: buffer=%d bytes chunk=%d bytes", settings.BufferBytes, chunkBytes)
	}
	return validateGains(config.Gains)
}

// ValidateTX проверяет общие и специфичные для UHD настройки передачи.
func ValidateTX(config device.TXConfig, settings TXSettings) error {
	if err := validateCommon(config.StreamConfig, config.RFAmplifierEnabled, config.AntennaPowerEnabled, config.HardwareTrigger); err != nil {
		return err
	}
	switch {
	case settings.Channel < 0:
		return errors.New("UHD TX channel must be >= 0")
	case settings.BufferLength <= 0:
		return errors.New("UHD buffer length must be > 0")
	case settings.ChunkSamples <= 0:
		return errors.New("UHD chunk size must be > 0")
	case settings.SettleDelay < 0:
		return errors.New("UHD settle delay must be >= 0")
	}
	return validateGains(config.Gains)
}

func validateCommon(config device.StreamConfig, rfAmplifier, antennaPower, hardwareTrigger bool) error {
	switch {
	case strings.TrimSpace(config.DeviceID) == "":
		return errors.New("UHD device args are required")
	case config.CenterFrequencyHz == 0:
		return errors.New("UHD center frequency must be > 0")
	case config.SampleRateHz == 0:
		return errors.New("UHD sample rate must be > 0")
	case config.BandwidthHz != 0:
		return errors.New("UHD bandwidth configuration is not supported")
	case config.SampleCount >= maximumSampleCount:
		return fmt.Errorf("UHD sample count must be less than %d", maximumSampleCount)
	case rfAmplifier:
		return errors.New("RF amplifier option is not supported by UHD adapter")
	case antennaPower:
		return errors.New("antenna power option is not supported by UHD adapter")
	case hardwareTrigger:
		return errors.New("hardware trigger option is not supported by UHD adapter")
	default:
		return nil
	}
}

func validateGains(gains []device.Gain) error {
	seen := make(map[string]struct{}, len(gains))
	for _, gain := range gains {
		stage := strings.ToUpper(strings.TrimSpace(gain.Stage))
		if stage == "" {
			return errors.New("UHD gain stage is required")
		}
		if _, duplicate := seen[stage]; duplicate {
			return fmt.Errorf("UHD gain stage %q is configured more than once", stage)
		}
		seen[stage] = struct{}{}
		if math.IsNaN(gain.ValueDB) || math.IsInf(gain.ValueDB, 0) {
			return fmt.Errorf("UHD gain %s must be finite", stage)
		}
	}
	return nil
}
