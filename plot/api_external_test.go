package plot_test

import (
	"testing"

	"github.com/GNSS-BANK/gnss-device-sdk/plot"
)

func TestPublicAPIRequiresOnlyPlotImport(t *testing.T) {
	chart, err := plot.New(
		plot.WithMinSize(plot.NewSize(800, 450)),
		plot.WithTheme(plot.ThemeDark),
		plot.WithFont(plot.FontGOSTTypeA),
		plot.WithFontSize(18),
		plot.WithBackend(plot.BackendAuto),
	)
	if err != nil {
		t.Fatal(err)
	}
	var publicChart plot.Chart = chart
	var windowConstructor func(plot.Chart, plot.WindowConfig) (plot.Window, error) = plot.NewWindow
	if publicChart == nil || windowConstructor == nil {
		t.Fatal("standalone plot API is not available")
	}
}
