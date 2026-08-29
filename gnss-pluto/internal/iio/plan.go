package iio

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

const (
	minFrequencyHz    uint64 = 325_000_000
	maxFrequencyHz    uint64 = 3_800_000_000
	minSampleRateHz   uint32 = 2_083_333
	maxSampleRateHz   uint32 = 61_440_000
	minBandwidthHz    uint32 = 200_000
	maxRXBandwidthHz  uint32 = 56_000_000
	maxTXBandwidthHz  uint32 = 40_000_000
	maxSampleCount    uint64 = 1 << 63
	minRXGainDB              = -3.0
	maxRXGainDB              = 71.0
	minTXGainDB              = -89.75
	maxTXGainDB              = 0.0
	hardwareGainStage        = "HARDWARE"
	physicalDevice           = "ad9361-phy"
	rxStreamDevice           = "cf-ad9361-lpc"
	txStreamDevice           = "cf-ad9361-dds-core-lpc"
)

// Binaries задаёт имена утилит libiio для плана операции.
type Binaries struct {
	Attr  string
	Read  string
	Write string
}

// Command описывает один последовательный вызов утилиты libiio.
type Command struct {
	Binary string
	Args   []string
}

// RXPlan проверяет конфигурацию и строит команды настройки и приёма.
func RXPlan(config device.RXConfig, binaries Binaries, bufferSize uint32) ([]Command, error) {
	uri, err := validateCommon(config.StreamConfig, config.RFAmplifierEnabled, config.AntennaPowerEnabled, config.HardwareTrigger, bufferSize)
	if err != nil {
		return nil, err
	}
	if config.BandwidthHz != 0 && (config.BandwidthHz < minBandwidthHz || config.BandwidthHz > maxRXBandwidthHz) {
		return nil, fmt.Errorf("RX bandwidth must be between %d and %d Hz", minBandwidthHz, maxRXBandwidthHz)
	}

	gain, configured, err := validateGain(config.Gains, minRXGainDB, maxRXGainDB, 1)
	if err != nil {
		return nil, err
	}
	if err := validateBinaries(binaries.Attr, binaries.Read, "iio_readdev"); err != nil {
		return nil, err
	}

	commands := make([]Command, 0, 6)
	commands = append(commands, attrCommand(binaries.Attr, uri, "-i", "voltage0", "sampling_frequency", strconv.FormatUint(uint64(config.SampleRateHz), 10)))
	if config.BandwidthHz != 0 {
		commands = append(commands, attrCommand(binaries.Attr, uri, "-i", "voltage0", "rf_bandwidth", strconv.FormatUint(uint64(config.BandwidthHz), 10)))
	}
	commands = append(commands, attrCommand(binaries.Attr, uri, "-o", "altvoltage0", "frequency", strconv.FormatUint(config.CenterFrequencyHz, 10)))
	if configured {
		commands = append(commands,
			attrCommand(binaries.Attr, uri, "-i", "voltage0", "gain_control_mode", "manual"),
			attrCommand(binaries.Attr, uri, "-i", "voltage0", "hardwaregain", formatGain(gain)),
		)
	}
	commands = append(commands, Command{
		Binary: binaries.Read,
		Args:   streamArgs(uri, bufferSize, config.SampleCount, rxStreamDevice),
	})
	return commands, nil
}

// TXPlan проверяет конфигурацию и строит команды настройки и передачи.
func TXPlan(config device.TXConfig, binaries Binaries, bufferSize uint32) ([]Command, error) {
	uri, err := validateCommon(config.StreamConfig, config.RFAmplifierEnabled, config.AntennaPowerEnabled, config.HardwareTrigger, bufferSize)
	if err != nil {
		return nil, err
	}
	if config.BandwidthHz != 0 && (config.BandwidthHz < minBandwidthHz || config.BandwidthHz > maxTXBandwidthHz) {
		return nil, fmt.Errorf("TX bandwidth must be between %d and %d Hz", minBandwidthHz, maxTXBandwidthHz)
	}

	gain, configured, err := validateGain(config.Gains, minTXGainDB, maxTXGainDB, 0.25)
	if err != nil {
		return nil, err
	}
	if err := validateBinaries(binaries.Attr, binaries.Write, "iio_writedev"); err != nil {
		return nil, err
	}

	commands := make([]Command, 0, 5)
	commands = append(commands, attrCommand(binaries.Attr, uri, "-o", "voltage0", "sampling_frequency", strconv.FormatUint(uint64(config.SampleRateHz), 10)))
	if config.BandwidthHz != 0 {
		commands = append(commands, attrCommand(binaries.Attr, uri, "-o", "voltage0", "rf_bandwidth", strconv.FormatUint(uint64(config.BandwidthHz), 10)))
	}
	commands = append(commands, attrCommand(binaries.Attr, uri, "-o", "altvoltage1", "frequency", strconv.FormatUint(config.CenterFrequencyHz, 10)))
	if configured {
		commands = append(commands, attrCommand(binaries.Attr, uri, "-o", "voltage0", "hardwaregain", formatGain(gain)))
	}
	commands = append(commands, Command{
		Binary: binaries.Write,
		Args:   streamArgs(uri, bufferSize, config.SampleCount, txStreamDevice),
	})
	return commands, nil
}

func validateCommon(config device.StreamConfig, rfAmplifier, antennaPower, hardwareTrigger bool, bufferSize uint32) (string, error) {
	uri := strings.TrimSpace(config.DeviceID)
	switch {
	case uri == "":
		return "", errors.New("device IIO URI is required")
	case config.CenterFrequencyHz < minFrequencyHz || config.CenterFrequencyHz > maxFrequencyHz:
		return "", fmt.Errorf("center frequency must be between %d and %d Hz", minFrequencyHz, maxFrequencyHz)
	case config.SampleRateHz < minSampleRateHz || config.SampleRateHz > maxSampleRateHz:
		return "", fmt.Errorf("sample rate must be between %d and %d Hz", minSampleRateHz, maxSampleRateHz)
	case config.SampleCount >= maxSampleCount:
		return "", fmt.Errorf("sample count must be less than %d", maxSampleCount)
	case bufferSize == 0:
		return "", errors.New("IIO buffer size must be greater than zero")
	case rfAmplifier:
		return "", errors.New("RF amplifier option is not supported by Pluto")
	case antennaPower:
		return "", errors.New("antenna power option is not supported by Pluto")
	case hardwareTrigger:
		return "", errors.New("hardware trigger option is not supported by Pluto")
	default:
		return uri, nil
	}
}

func validateGain(gains []device.Gain, minimum, maximum, step float64) (float64, bool, error) {
	if len(gains) == 0 {
		return 0, false, nil
	}
	if len(gains) > 1 {
		return 0, false, errors.New("gain stage HARDWARE can be configured only once")
	}

	gain := gains[0]
	stage := strings.ToUpper(strings.TrimSpace(gain.Stage))
	if stage == "" {
		return 0, false, errors.New("gain stage is required")
	}
	if stage != hardwareGainStage {
		return 0, false, fmt.Errorf("unsupported gain stage %q", gain.Stage)
	}
	if math.IsNaN(gain.ValueDB) || math.IsInf(gain.ValueDB, 0) || gain.ValueDB < minimum || gain.ValueDB > maximum {
		return 0, false, fmt.Errorf("gain HARDWARE must be between %g and %g dB", minimum, maximum)
	}
	steps := (gain.ValueDB - minimum) / step
	if math.Abs(steps-math.Round(steps)) > 1e-9 {
		return 0, false, fmt.Errorf("gain HARDWARE must use a %g dB step", step)
	}
	return gain.ValueDB, true, nil
}

func validateBinaries(attrBinary, streamBinary, streamName string) error {
	if strings.TrimSpace(attrBinary) == "" {
		return errors.New("iio_attr binary is required")
	}
	if strings.TrimSpace(streamBinary) == "" {
		return fmt.Errorf("%s binary is required", streamName)
	}
	return nil
}

func attrCommand(binary, uri, direction, channel, attribute, value string) Command {
	return Command{
		Binary: binary,
		Args: []string{
			"-u", uri,
			direction,
			"-c", physicalDevice,
			channel,
			attribute,
			value,
		},
	}
}

func streamArgs(uri string, bufferSize uint32, sampleCount uint64, streamDevice string) []string {
	args := []string{
		"-u", uri,
		"-b", strconv.FormatUint(uint64(bufferSize), 10),
	}
	if sampleCount != 0 {
		args = append(args, "-s", strconv.FormatUint(sampleCount, 10))
	}
	return append(args, streamDevice, "voltage0", "voltage1")
}

func formatGain(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
