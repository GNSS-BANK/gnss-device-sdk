package transfer

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

const (
	minFrequencyHz  uint64 = 1_000_000
	maxFrequencyHz  uint64 = 6_000_000_000
	minSampleRateHz uint32 = 2_000_000
	maxSampleRateHz uint32 = 20_000_000
	maxSampleCount  uint64 = 1 << 63
)

var supportedBandwidthsHz = []uint32{
	1_750_000,
	2_500_000,
	3_500_000,
	5_000_000,
	5_500_000,
	6_000_000,
	7_000_000,
	8_000_000,
	9_000_000,
	10_000_000,
	12_000_000,
	14_000_000,
	15_000_000,
	20_000_000,
	24_000_000,
	28_000_000,
}

type gainRule struct {
	defaultValue uint32
	maximum      uint32
	step         uint32
}

var rxGainRules = map[string]gainRule{
	"LNA": {defaultValue: 8, maximum: 40, step: 8},
	"VGA": {defaultValue: 20, maximum: 62, step: 2},
}

var txGainRules = map[string]gainRule{
	"VGA": {defaultValue: 0, maximum: 47, step: 1},
}

func RXArgs(config device.RXConfig) ([]string, error) {
	common, err := commonArgs(config.StreamConfig)
	if err != nil {
		return nil, err
	}

	gains, err := validateGains(config.Gains, rxGainRules)
	if err != nil {
		return nil, err
	}

	args := []string{"-r", "-"}
	args = append(args, common...)
	args = append(args,
		"-a", boolArgument(config.RFAmplifierEnabled),
		"-p", boolArgument(config.AntennaPowerEnabled),
		"-l", strconv.FormatUint(uint64(gains["LNA"]), 10),
		"-g", strconv.FormatUint(uint64(gains["VGA"]), 10),
	)
	if config.HardwareTrigger {
		args = append(args, "-H")
	}

	return args, nil
}

func TXArgs(config device.TXConfig) ([]string, error) {
	common, err := commonArgs(config.StreamConfig)
	if err != nil {
		return nil, err
	}

	gains, err := validateGains(config.Gains, txGainRules)
	if err != nil {
		return nil, err
	}

	args := []string{"-t", "-"}
	args = append(args, common...)
	args = append(args,
		"-a", boolArgument(config.RFAmplifierEnabled),
		"-p", boolArgument(config.AntennaPowerEnabled),
		"-x", strconv.FormatUint(uint64(gains["VGA"]), 10),
	)
	if config.HardwareTrigger {
		args = append(args, "-H")
	}

	return args, nil
}

func commonArgs(config device.StreamConfig) ([]string, error) {
	switch {
	case config.CenterFrequencyHz < minFrequencyHz || config.CenterFrequencyHz > maxFrequencyHz:
		return nil, fmt.Errorf(
			"center frequency must be between %d and %d Hz",
			minFrequencyHz,
			maxFrequencyHz,
		)
	case config.SampleRateHz < minSampleRateHz || config.SampleRateHz > maxSampleRateHz:
		return nil, fmt.Errorf(
			"sample rate must be between %d and %d Hz",
			minSampleRateHz,
			maxSampleRateHz,
		)
	case config.BandwidthHz != 0 && !slices.Contains(supportedBandwidthsHz, config.BandwidthHz):
		return nil, fmt.Errorf("unsupported baseband bandwidth %d Hz", config.BandwidthHz)
	case config.SampleCount >= maxSampleCount:
		return nil, fmt.Errorf("sample count must be less than %d", maxSampleCount)
	}

	args := make([]string, 0, 12)
	if deviceID := strings.TrimSpace(config.DeviceID); deviceID != "" {
		args = append(args, "-d", deviceID)
	}
	args = append(args,
		"-f", strconv.FormatUint(config.CenterFrequencyHz, 10),
		"-s", strconv.FormatUint(uint64(config.SampleRateHz), 10),
	)
	if config.BandwidthHz != 0 {
		args = append(args, "-b", strconv.FormatUint(uint64(config.BandwidthHz), 10))
	}
	if config.SampleCount != 0 {
		args = append(args, "-n", strconv.FormatUint(config.SampleCount, 10))
	}

	return args, nil
}

func validateGains(configured []device.Gain, rules map[string]gainRule) (map[string]uint32, error) {
	values := make(map[string]uint32, len(rules))
	for stage, rule := range rules {
		values[stage] = rule.defaultValue
	}

	seen := make(map[string]struct{}, len(configured))
	for _, gain := range configured {
		stage := strings.ToUpper(strings.TrimSpace(gain.Stage))
		if stage == "" {
			return nil, errors.New("gain stage is required")
		}
		rule, ok := rules[stage]
		if !ok {
			return nil, fmt.Errorf("unsupported gain stage %q", gain.Stage)
		}
		if _, duplicate := seen[stage]; duplicate {
			return nil, fmt.Errorf("gain stage %q is configured more than once", stage)
		}
		seen[stage] = struct{}{}

		if math.IsNaN(gain.ValueDB) || math.IsInf(gain.ValueDB, 0) || gain.ValueDB < 0 || math.Trunc(gain.ValueDB) != gain.ValueDB {
			return nil, fmt.Errorf("gain %s must be a non-negative integer", stage)
		}
		if gain.ValueDB > float64(rule.maximum) {
			return nil, fmt.Errorf("gain %s must not exceed %d dB", stage, rule.maximum)
		}
		value := uint32(gain.ValueDB)
		if value%rule.step != 0 {
			return nil, fmt.Errorf("gain %s must use a %d dB step", stage, rule.step)
		}
		values[stage] = value
	}

	return values, nil
}

func boolArgument(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
