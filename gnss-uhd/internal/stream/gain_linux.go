//go:build linux && cgo

package stream

import (
	"fmt"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	sdrlib "hz.tools/sdr"
	"hz.tools/sdr/uhd"
)

func applyGains(conn *uhd.Sdr, configured []device.Gain, stageType sdrlib.GainStageType) error {
	if len(configured) == 0 {
		return nil
	}

	stages, err := conn.GetGainStages()
	if err != nil {
		return fmt.Errorf("failed to read UHD gain stages: %w", err)
	}
	filtered := stages.Filter(stageType)
	available := gainStageNames(filtered)
	if len(filtered) == 0 {
		return fmt.Errorf("UHD does not expose %s gain stages", stageType.String())
	}

	for _, setting := range configured {
		stage, err := findGainStage(filtered, setting.Stage)
		if err != nil {
			return fmt.Errorf("%w; available gain stages: %s", err, strings.Join(available, ", "))
		}
		rangeDB := stage.Range()
		if setting.ValueDB < float64(rangeDB[0]) || setting.ValueDB > float64(rangeDB[1]) {
			return fmt.Errorf("UHD gain %s must be between %g and %g dB", stage.String(), rangeDB[0], rangeDB[1])
		}
		if err := conn.SetGain(stage, float32(setting.ValueDB)); err != nil {
			return fmt.Errorf("failed to set UHD gain %s=%g dB: %w", stage.String(), setting.ValueDB, err)
		}
	}
	return nil
}
