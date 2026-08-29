package plot

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestStreamingPauseAndLimit(t *testing.T) {
	chart, err := newPlot(WithMaxPoints(3))
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
	chart, err := newPlot(WithBackend(BackendCPU))
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

func TestDragMovesViewport(t *testing.T) {
	chart, err := newPlot()
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
	plotWidth := chart.Size().Width - plotMarginLeft - plotMarginRight
	plotHeight := chart.Size().Height - plotMarginTop - plotMarginBottom
	chart.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{
		DX: plotWidth / 10,
		DY: plotHeight / 5,
	}})
	view := chart.snapshot().view
	if math.Abs(view.xMin-(-1)) > 0.0001 || math.Abs(view.xMax-9) > 0.0001 {
		t.Fatalf("unexpected X range after drag: %#v", view)
	}
	if math.Abs(view.yMin-2) > 0.0001 || math.Abs(view.yMax-12) > 0.0001 {
		t.Fatalf("unexpected Y range after drag: %#v", view)
	}
}

func TestThemeOptionAndControls(t *testing.T) {
	chart, err := newPlot(WithTheme(ThemeLight))
	if err != nil {
		t.Fatal(err)
	}
	if chart.snapshot().theme != ThemeLight {
		t.Fatal("WithTheme did not configure the initial theme")
	}
	if _, err := newPlot(WithTheme(ThemeVariant(99))); err == nil {
		t.Fatal("expected invalid theme error")
	}

	minimal, err := newControls(chart, ControlsConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(minimal.Objects) != 2 {
		t.Fatalf("minimal controls must contain only renderer label and selector, got %d objects", len(minimal.Objects))
	}

	controls, err := newControls(chart, ControlsConfig{
		ShowPause:     true,
		ShowZoom:      true,
		ShowClear:     true,
		ShowResetZoom: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(controls.Objects) != 8 {
		t.Fatalf("unexpected full controls object count: %d", len(controls.Objects))
	}
	for _, object := range controls.Objects {
		switch current := object.(type) {
		case *widget.Button:
			if current.Text == "Пауза" {
				test.Tap(current)
				if !chart.Paused() || current.Text != "Продолжить" {
					t.Fatal("pause control did not toggle chart state")
				}
			}
		case *widget.Select:
			current.SetSelected("CPU")
		}
	}
	if chart.Backend() != BackendCPU {
		t.Fatal("renderer selector did not change backend")
	}
	if _, err := newControls(chart, ControlsConfig{ZoomFactor: 2}); err == nil {
		t.Fatal("expected invalid controls zoom factor error")
	}
	var nilChart *plotWidget
	if _, err := newControls(nilChart, ControlsConfig{}); err == nil {
		t.Fatal("expected nil chart error")
	}
}

func TestFontOptionAndGOSTResource(t *testing.T) {
	chart, err := newPlot(WithFont(FontGOSTTypeA), WithFontSize(18))
	if err != nil {
		t.Fatal(err)
	}
	if chart.snapshot().font != FontGOSTTypeA {
		t.Fatal("WithFont did not configure GOST Type A")
	}
	if chart.snapshot().fontSize != 18 {
		t.Fatal("WithFontSize did not configure the base font size")
	}
	if _, err := newPlot(WithFont(FontFamily(99))); err == nil {
		t.Fatal("expected invalid font error")
	}
	if len(gostTypeAResource.Content()) == 0 {
		t.Fatal("embedded GOST Type A resource is empty")
	}

	face, err := newVerticalTextFace(FontGOSTTypeA, 13)
	if err != nil {
		t.Fatalf("parse embedded GOST Type A: %v", err)
	}
	if _, ok := face.GlyphAdvance('А'); !ok {
		t.Fatal("embedded GOST Type A does not contain Cyrillic glyphs")
	}
	_ = face.Close()

	state := &verticalTextState{
		text:  "Амплитуда",
		color: color.White,
		font:  FontGOSTTypeA,
	}
	if countOpaque(state.render(24, 200)) == 0 {
		t.Fatal("GOST Type A vertical label produced an empty image")
	}

	window, err := NewWindow(chart, WindowConfig{
		Title: "GOST font test",
		Size:  NewSize(640, 360),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		window.Close()
		test.NewApp()
	}()
	actual := window.(*plotWindow).application.Settings().Theme().Font(fyne.TextStyle{})
	if actual.Name() != gostTypeAResource.Name() {
		t.Fatalf("unexpected application font: %q", actual.Name())
	}
	actualSize := window.(*plotWindow).application.Settings().Theme().Size(fynetheme.SizeNameText)
	if actualSize != 18 {
		t.Fatalf("unexpected application font size: %v", actualSize)
	}
}

func TestFontSizeScalesRendererAndInteraction(t *testing.T) {
	chart, err := newPlot(WithFontSize(28), WithBackend(BackendCPU))
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []float32{0, 5, 73, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := newPlot(WithFontSize(invalid)); err == nil {
			t.Fatalf("expected invalid font size error for %v", invalid)
		}
	}

	chart.Resize(fyne.NewSize(900, 600))
	if err := chart.SetAxes(AxesConfig{
		X: AxisConfig{Fixed: true, Min: 0, Max: 10, Ticks: 3},
		Y: AxisConfig{Fixed: true, Min: 0, Max: 10, Ticks: 3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := chart.SetSeries([]Series{{
		ID:     "signal",
		Points: []Point{{X: 5, Y: 5}},
	}}); err != nil {
		t.Fatal(err)
	}

	renderer := chart.CreateRenderer().(*plotRenderer)
	if renderer.labelX.TextSize != 28 || renderer.tickX[0].TextSize != 22 || renderer.legendTexts[0].TextSize != 24 {
		t.Fatalf("unexpected scaled font sizes: label=%v tick=%v legend=%v",
			renderer.labelX.TextSize,
			renderer.tickX[0].TextSize,
			renderer.legendTexts[0].TextSize,
		)
	}
	if renderer.labelY.Size().Width != 48 {
		t.Fatalf("unexpected scaled vertical label width: %v", renderer.labelY.Size().Width)
	}

	metrics := newPlotLayoutMetrics(28)
	plotWidth := chart.Size().Width - metrics.marginLeft - metrics.marginRight
	plotHeight := chart.Size().Height - metrics.marginTop - metrics.marginBottom
	chart.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(
		metrics.marginLeft+plotWidth/2,
		metrics.marginTop+plotHeight/2,
	)}})
	if hover := chart.snapshot().hover; hover == nil || hover.point != (Point{X: 5, Y: 5}) {
		t.Fatalf("scaled layout hover did not select the point: %#v", hover)
	}

	largeChart, err := newPlot(WithFontSize(maxFontSize))
	if err != nil {
		t.Fatal(err)
	}
	largeMinimum := largeChart.CreateRenderer().MinSize()
	if largeMinimum.Width <= 640 || largeMinimum.Height <= 360 {
		t.Fatalf("large font did not expand minimum size: %#v", largeMinimum)
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

func TestShaderSourcesDoNotUseReservedPackedIdentifier(t *testing.T) {
	for name, source := range map[string]string{
		"desktop": plotShaderSource,
		"es":      plotShaderSourceES,
	} {
		if strings.Contains(source, " packed ") {
			t.Fatalf("%s shader uses reserved GLSL identifier packed", name)
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

func TestVerticalAxisLabelRaster(t *testing.T) {
	state := &verticalTextState{
		text:  "Амплитуда",
		color: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	}
	result := state.render(24, 200)
	if countOpaque(result) == 0 {
		t.Fatal("vertical axis label produced an empty image")
	}
	minimumX, minimumY := result.Bounds().Max.X, result.Bounds().Max.Y
	maximumX, maximumY := result.Bounds().Min.X, result.Bounds().Min.Y
	for y := result.Bounds().Min.Y; y < result.Bounds().Max.Y; y++ {
		for x := result.Bounds().Min.X; x < result.Bounds().Max.X; x++ {
			_, _, _, alpha := result.At(x, y).RGBA()
			if alpha == 0 {
				continue
			}
			minimumX = min(minimumX, x)
			maximumX = max(maximumX, x)
			minimumY = min(minimumY, y)
			maximumY = max(maximumY, y)
		}
	}
	if maximumY-minimumY <= maximumX-minimumX {
		t.Fatalf("axis label is not vertical: x=%d..%d y=%d..%d", minimumX, maximumX, minimumY, maximumY)
	}
}

func TestRendererSelectsCPUAndGPU(t *testing.T) {
	for _, backend := range []RenderBackend{BackendCPU, BackendGPU} {
		t.Run(backendName(backend), func(t *testing.T) {
			chart, err := newPlot(WithBackend(backend), WithMaxSeries(2))
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
	chart, err := newPlot(WithBackend(BackendCPU))
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

	window, err := NewWindow(chart, WindowConfig{
		Title: "Capture test",
		Size:  NewSize(640, 360),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	result, err := window.Capture()
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
