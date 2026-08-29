package stream

import (
	"fmt"
	"slices"
	"strings"

	sdrlib "hz.tools/sdr"
)

func findGainStage(stages sdrlib.GainStages, requested string) (sdrlib.GainStage, error) {
	needle := strings.ToUpper(strings.TrimSpace(requested))
	if needle == "" {
		return nil, fmt.Errorf("gain stage name is empty")
	}
	for _, stage := range stages {
		if strings.EqualFold(stage.String(), needle) {
			return stage, nil
		}
	}

	matches := make(sdrlib.GainStages, 0, len(stages))
	for _, stage := range stages {
		if strings.HasSuffix(strings.ToUpper(stage.String()), needle) {
			matches = append(matches, stage)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("gain stage %q not found", requested)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("gain stage %q is ambiguous: %s", requested, strings.Join(gainStageNames(matches), ", "))
	}
}

func gainStageNames(stages sdrlib.GainStages) []string {
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, stage.String())
	}
	slices.Sort(names)
	return names
}
