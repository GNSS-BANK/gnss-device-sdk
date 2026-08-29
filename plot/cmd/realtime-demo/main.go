package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/GNSS-BANK/gnss-device-sdk/plot"
)

func main() {
	application := app.New()
	window := application.NewWindow("GNSS realtime plot demo")
	window.Resize(fyne.NewSize(1100, 650))
	window.CenterOnScreen()

	chart, err := plot.New(
		plot.WithMaxPoints(1200),
		plot.WithMaxSeries(4),
		plot.WithBackend(plot.BackendAuto),
		plot.WithTheme(plot.ThemeDark),
		plot.WithMinSize(fyne.NewSize(800, 450)),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = chart.SetSeries([]plot.Series{
		{
			ID:          "sin",
			Name:        "sin(t)",
			Mode:        plot.DrawLine,
			Color:       color.NRGBA{R: 68, G: 138, B: 255, A: 255},
			LineWidth:   2,
			PointRadius: 3,
		},
		{
			ID:          "cos",
			Name:        "cos(t)",
			Mode:        plot.DrawPoints,
			Color:       color.NRGBA{R: 255, G: 105, B: 97, A: 255},
			PointRadius: 3,
		},
		{
			ID:          "noise",
			Name:        "Шум",
			Mode:        plot.DrawPopsicle,
			Color:       color.NRGBA{R: 71, G: 201, B: 142, A: 210},
			LineWidth:   1,
			PointRadius: 2,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	err = chart.SetAxes(plot.AxesConfig{
		X: plot.AxisConfig{
			Label: "Время, с",
			Ticks: 8,
			Formatter: func(value float64) string {
				return fmt.Sprintf("%.1f", value)
			},
		},
		Y: plot.AxisConfig{
			Label: "Амплитуда",
			Fixed: true,
			Min:   -1.5,
			Max:   1.5,
			Ticks: 7,
			Formatter: func(value float64) string {
				return fmt.Sprintf("%.1f", value)
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	chart.SetLegendVisible(true)

	view, err := plot.NewView(chart, plot.ControlsConfig{
		ShowPause:     true,
		ShowZoom:      true,
		ShowClear:     true,
		ShowResetZoom: true,
		ZoomFactor:    0.8,
	})
	if err != nil {
		log.Fatal(err)
	}
	window.SetContent(view)

	done := make(chan struct{})
	window.SetOnClosed(func() {
		close(done)
	})

	go generateRealtimeData(chart, done)

	window.ShowAndRun()
}

func generateRealtimeData(chart plot.Chart, done <-chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	startedAt := time.Now()
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case now := <-ticker.C:
			elapsed := now.Sub(startedAt).Seconds()

			sinValue := math.Sin(elapsed * 2)
			cosValue := math.Cos(elapsed * 1.3)
			noiseValue := (random.Float64()*2 - 1) * 0.4

			if err := chart.Append(
				"sin",
				plot.Point{X: elapsed, Y: sinValue},
			); err != nil {
				log.Printf("append sin: %v", err)
			}

			if err := chart.Append(
				"cos",
				plot.Point{X: elapsed, Y: cosValue},
			); err != nil {
				log.Printf("append cos: %v", err)
			}

			if err := chart.Append(
				"noise",
				plot.Point{X: elapsed, Y: noiseValue},
			); err != nil {
				log.Printf("append noise: %v", err)
			}

		case <-done:
			return
		}
	}
}
