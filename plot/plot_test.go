package plot

import (
	"image"
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func TestStreamingPauseAndLimit(t *testing.T) {
	chart, err := New(WithMaxPoints(3))
	if err != nil {
		t.Fatal(err)
	}
	err = chart.SetSeries([]Series{{
		ID:     "signal",
		Points: []Point{{X: 3, Y: 3}, {X: 1, Y: 1}, {X: 2, Y: 2}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	chart.Pause()
	if err := chart.Append("signal", Point{X: 4, Y: 4}, Point{X: 5, Y: 5}); err != nil {
		t.Fatal(err)
	}
	frozen := chart.snapshot().series[0].Points
	if len(frozen) != 3 || frozen[0].X != 1 || frozen[2].X != 3 {
		t.Fatalf("unexpected frozen points: %#v", frozen)
	}
	chart.Resume()
	current := chart.snapshot().series[0].Points
	if len(current) != 3 || current[0].X != 3 || current[2].X != 5 {
		t.Fatalf("unexpected sliding window: %#v", current)
	}
	if err := chart.Append("signal", Point{X: 4, Y: 0}); err == nil {
		t.Fatal("expected monotonically increasing X error")
	}
}

func TestAxesZoomBackendAndClear(t *testing.T) {
	chart, err := New(WithBackend(BackendCPU))
	if err != nil {
		t.Fatal(err)
	}
	if chart.Backend() != BackendCPU {
		t.Fatalf("unexpected backend: %v", chart.Backend())
	}
	if err := chart.SetBackend(RenderBackend(99)); err == nil {
		t.Fatal("expected invalid backend error")
	}
	if err := chart.SetAxes(AxesConfig{
		X: AxisConfig{Fixed: true, Min: 0, Max: 10, Ticks: 3},
		Y: AxisConfig{Fixed: true, Min: -2, Max: 2, Ticks: 5},
	}); err != nil {
		t.Fatal(err)
	}
	before := chart.snapshot().view
	if err := chart.Zoom(0.5); err != nil {
		t.Fatal(err)
	}
	after := chart.snapshot().view
	if after.xMax-after.xMin >= before.xMax-before.xMin {
		t.Fatalf("zoom did not reduce X range: before=%#v after=%#v", before, after)
	}
	chart.ResetZoom()
	if chart.snapshot().view != before {
		t.Fatalf("reset zoom did not restore configured range")
	}
	if err := chart.SetSeries([]Series{{ID: "a", Points: []Point{{X: 1, Y: 1}}}}); err != nil {
		t.Fatal(err)
	}
	chart.Clear()
	if len(chart.snapshot().series[0].Points) != 0 {
		t.Fatal("clear left points in the series")
	}
}

func TestPointTextureRoundTrip(t *testing.T) {
	points := []Point{{X: -1, Y: 2}, {X: 0.25, Y: 0.75}, {X: 2, Y: -1}}
	encoded := encodePoints(points)
	for index, expected := range points {
		offset := encoded.PixOffset(index, 0)
		x := uint16(encoded.Pix[offset])<<8 | uint16(encoded.Pix[offset+1])
		y := uint16(encoded.Pix[offset+2])<<8 | uint16(encoded.Pix[offset+3])
		decodedX := float64(x)/65535*encodedCoordinateScale - encodedCoordinateOffset
		decodedY := float64(y)/65535*encodedCoordinateScale - encodedCoordinateOffset
		if math.Abs(decodedX-expected.X) > 0.0001 || math.Abs(decodedY-expected.Y) > 0.0001 {
			t.Fatalf("point %d changed: expected=%#v got=(%f,%f)", index, expected, decodedX, decodedY)
		}
	}
}

func TestCPURendererSupportsAllDrawModes(t *testing.T) {
	for _, mode := range []DrawMode{DrawLine, DrawPoints, DrawPopsicle} {
		t.Run(drawModeName(mode), func(t *testing.T) {
			state := &cpuRasterState{frame: cpuFrame{
				view: axisRange{xMin: 0, xMax: 1, yMin: 0, yMax: 1},
				series: []Series{{
					ID:          "signal",
					Mode:        mode,
					Color:       color.NRGBA{R: 255, A: 255},
					LineWidth:   2,
					PointRadius: 4,
					Points:      []Point{{X: 0.2, Y: 0.2}, {X: 0.8, Y: 0.8}},
				}},
			}}
			result := state.render(100, 80)
			if countOpaque(result) == 0 {
				t.Fatalf("CPU renderer produced an empty image for mode %v", mode)
			}
		})
	}
}

func TestRendererSelectsCPUAndGPU(t *testing.T) {
	for _, backend := range []RenderBackend{BackendCPU, BackendGPU} {
		t.Run(backendName(backend), func(t *testing.T) {
			chart, err := New(WithBackend(backend), WithMaxSeries(2))
			if err != nil {
				t.Fatal(err)
			}
			chart.Resize(fyne.NewSize(640, 360))
			if err := chart.SetSeries([]Series{{ID: "signal", Points: []Point{{X: 0, Y: 0}, {X: 1, Y: 1}}}}); err != nil {
				t.Fatal(err)
			}
			renderer := chart.CreateRenderer().(*plotRenderer)
			renderer.Refresh()
			if backend == BackendCPU {
				if !renderer.raster.Visible() || renderer.shaders[0].Visible() {
					t.Fatal("CPU backend visibility is incorrect")
				}
			} else {
				if renderer.raster.Visible() || !renderer.shaders[0].Visible() {
					t.Fatal("GPU backend visibility is incorrect")
				}
				if renderer.shaders[0].Uniforms["pointCount"] != 2 {
					t.Fatal("GPU shader did not receive series points")
				}
			}
		})
	}
}

func TestHoverAndCapture(t *testing.T) {
	chart, err := New(WithBackend(BackendCPU))
	if err != nil {
		t.Fatal(err)
	}
	chart.Resize(fyne.NewSize(640, 360))
	if err := chart.SetAxes(AxesConfig{
		X: AxisConfig{Fixed: true, Min: 0, Max: 10, Ticks: 3},
		Y: AxisConfig{Fixed: true, Min: 0, Max: 10, Ticks: 3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := chart.SetSeries([]Series{{ID: "signal", Name: "Сигнал", Points: []Point{{X: 5, Y: 5}}}}); err != nil {
		t.Fatal(err)
	}
	plotWidth := chart.Size().Width - plotMarginLeft - plotMarginRight
	plotHeight := chart.Size().Height - plotMarginTop - plotMarginBottom
	chart.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(
		plotMarginLeft+plotWidth/2,
		plotMarginTop+plotHeight/2,
	)}})
	if hover := chart.snapshot().hover; hover == nil || hover.point != (Point{X: 5, Y: 5}) {
		t.Fatalf("hover did not select the point: %#v", hover)
	}
	chart.MouseOut()
	if chart.snapshot().hover != nil {
		t.Fatal("MouseOut did not clear hover")
	}

	canvas := test.NewCanvas()
	canvas.SetContent(chart)
	canvas.Resize(fyne.NewSize(640, 360))
	result, err := chart.Capture(canvas)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bounds().Empty() {
		t.Fatal("capture returned an empty image")
	}
}

func countOpaque(value image.Image) int {
	count := 0
	for y := value.Bounds().Min.Y; y < value.Bounds().Max.Y; y++ {
		for x := value.Bounds().Min.X; x < value.Bounds().Max.X; x++ {
			_, _, _, alpha := value.At(x, y).RGBA()
			if alpha != 0 {
				count++
			}
		}
	}
	return count
}

func drawModeName(mode DrawMode) string {
	return []string{"line", "points", "popsicle"}[mode]
}

func backendName(backend RenderBackend) string {
	return []string{"auto", "gpu", "cpu"}[backend]
}
