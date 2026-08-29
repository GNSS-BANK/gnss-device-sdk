package stream

import (
	"strings"
	"testing"

	sdrlib "hz.tools/sdr"
)

type testGainStage struct {
	name      string
	stageType sdrlib.GainStageType
}

func (stage testGainStage) String() string             { return stage.name }
func (stage testGainStage) Type() sdrlib.GainStageType { return stage.stageType }
func (stage testGainStage) Range() [2]float32          { return [2]float32{-10, 70} }

func TestFindGainStage(t *testing.T) {
	rx0 := testGainStage{name: "RX0PGA", stageType: sdrlib.GainStageTypeRecieve}
	rx1 := testGainStage{name: "RX1PGA", stageType: sdrlib.GainStageTypeRecieve}
	stages := sdrlib.GainStages{rx0, rx1}

	t.Run("exact name", func(t *testing.T) {
		got, err := findGainStage(stages, "rx0pga")
		if err != nil || got.String() != rx0.name {
			t.Fatalf("findGainStage() = %v, %v", got, err)
		}
	})

	t.Run("unique suffix", func(t *testing.T) {
		got, err := findGainStage(sdrlib.GainStages{rx0}, "PGA")
		if err != nil || got.String() != rx0.name {
			t.Fatalf("findGainStage() = %v, %v", got, err)
		}
	})

	t.Run("ambiguous suffix", func(t *testing.T) {
		_, err := findGainStage(stages, "PGA")
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("findGainStage() error = %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := findGainStage(stages, "LNA")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("findGainStage() error = %v", err)
		}
	})
}

func TestGainStageNamesSortsNames(t *testing.T) {
	names := gainStageNames(sdrlib.GainStages{
		testGainStage{name: "RX1PGA"},
		testGainStage{name: "RX0PGA"},
	})
	if strings.Join(names, ",") != "RX0PGA,RX1PGA" {
		t.Fatalf("gainStageNames() = %v", names)
	}
}
