package plot

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

const (
	plotMarginLeft   float32 = 92
	plotMarginRight  float32 = 20
	plotMarginTop    float32 = 20
	plotMarginBottom float32 = 58
)

type palette struct {
	background     color.Color
	plotBackground color.Color
	foreground     color.Color
	grid           color.Color
	legend         color.Color
	tooltip        color.Color
	tooltipText    color.Color
	marker         color.Color
}

type plotRenderer struct {
	plot *Plot

	background     *canvas.Rectangle
	plotBackground *canvas.Rectangle
	gridX          []*canvas.Line
	gridY          []*canvas.Line
	axisX          *canvas.Line
	axisY          *canvas.Line
	tickX          []*canvas.Text
	tickY          []*canvas.Text
	labelX         *canvas.Text
	labelY         *canvas.Raster
	labelYState    *verticalTextState
	raster         *canvas.Raster
	rasterState    *cpuRasterState
	shaders        []*canvas.Shader
	legendBox      *canvas.Rectangle
	legendSwatches []*canvas.Rectangle
	legendTexts    []*canvas.Text
	hoverMarker    *canvas.Circle
	hoverBox       *canvas.Rectangle
	hoverText      *canvas.Text
	objects        []fyne.CanvasObject
	gpuConfigured  bool
	gpuRevision    uint64
	cpuConfigured  bool
	cpuRevision    uint64
}

// CreateRenderer реализует fyne.Widget и создаёт GPU/CPU renderer графика.
func (plot *Plot) CreateRenderer() fyne.WidgetRenderer {
	renderer := &plotRenderer{plot: plot}
	renderer.background = canvas.NewRectangle(color.Transparent)
	renderer.plotBackground = canvas.NewRectangle(color.Transparent)
	renderer.objects = append(renderer.objects, renderer.background, renderer.plotBackground)

	for index := 0; index < maxAxisTicks; index++ {
		line := canvas.NewLine(color.Transparent)
		line.StrokeWidth = 1
		renderer.gridX = append(renderer.gridX, line)
		renderer.objects = append(renderer.objects, line)
	}
	for index := 0; index < maxAxisTicks; index++ {
		line := canvas.NewLine(color.Transparent)
		line.StrokeWidth = 1
		renderer.gridY = append(renderer.gridY, line)
		renderer.objects = append(renderer.objects, line)
	}

	renderer.raster, renderer.rasterState = newCPURaster()
	renderer.objects = append(renderer.objects, renderer.raster)
	for index := 0; index < plot.maxSeries; index++ {
		shader := newSeriesShader(index)
		renderer.shaders = append(renderer.shaders, shader)
		renderer.objects = append(renderer.objects, shader)
	}

	renderer.axisX = canvas.NewLine(color.Transparent)
	renderer.axisY = canvas.NewLine(color.Transparent)
	renderer.axisX.StrokeWidth = 1
	renderer.axisY.StrokeWidth = 1
	renderer.objects = append(renderer.objects, renderer.axisX, renderer.axisY)
	for index := 0; index < maxAxisTicks; index++ {
		label := canvas.NewText("", color.Transparent)
		label.TextSize = 11
		label.Alignment = fyne.TextAlignCenter
		renderer.tickX = append(renderer.tickX, label)
		renderer.objects = append(renderer.objects, label)
	}
	for index := 0; index < maxAxisTicks; index++ {
		label := canvas.NewText("", color.Transparent)
		label.TextSize = 11
		label.Alignment = fyne.TextAlignTrailing
		renderer.tickY = append(renderer.tickY, label)
		renderer.objects = append(renderer.objects, label)
	}
	renderer.labelX = canvas.NewText("", color.Transparent)
	renderer.labelX.Alignment = fyne.TextAlignCenter
	renderer.labelX.TextStyle = fyne.TextStyle{Bold: true}
	renderer.labelY, renderer.labelYState = newVerticalText()
	renderer.objects = append(renderer.objects, renderer.labelX, renderer.labelY)

	renderer.legendBox = canvas.NewRectangle(color.Transparent)
	renderer.legendBox.CornerRadius = 5
	renderer.objects = append(renderer.objects, renderer.legendBox)
	for index := 0; index < plot.maxSeries; index++ {
		swatch := canvas.NewRectangle(color.Transparent)
		text := canvas.NewText("", color.Transparent)
		text.TextSize = 12
		renderer.legendSwatches = append(renderer.legendSwatches, swatch)
		renderer.legendTexts = append(renderer.legendTexts, text)
		renderer.objects = append(renderer.objects, swatch, text)
	}

	renderer.hoverMarker = canvas.NewCircle(color.Transparent)
	renderer.hoverMarker.StrokeWidth = 2
	renderer.hoverBox = canvas.NewRectangle(color.Transparent)
	renderer.hoverBox.CornerRadius = 4
	renderer.hoverText = canvas.NewText("", color.Transparent)
	renderer.hoverText.TextSize = 12
	renderer.objects = append(renderer.objects, renderer.hoverMarker, renderer.hoverBox, renderer.hoverText)
	renderer.Refresh()
	return renderer
}

func (renderer *plotRenderer) Destroy() {}

func (renderer *plotRenderer) Layout(size fyne.Size) {
	snapshot := renderer.plot.snapshot()
	renderer.layout(size, snapshot)
}

func (renderer *plotRenderer) MinSize() fyne.Size {
	return renderer.plot.snapshot().minimumSize
}

func (renderer *plotRenderer) Objects() []fyne.CanvasObject {
	return renderer.objects
}

func (renderer *plotRenderer) Refresh() {
	snapshot := renderer.plot.snapshot()
	colors := themePalette(snapshot.theme)
	renderer.background.FillColor = colors.background
	renderer.plotBackground.FillColor = colors.plotBackground
	renderer.axisX.StrokeColor = colors.foreground
	renderer.axisY.StrokeColor = colors.foreground
	renderer.labelX.Color = colors.foreground
	renderer.labelX.Text = snapshot.axes.X.Label
	if renderer.labelYState.set(snapshot.axes.Y.Label, colors.foreground) {
		renderer.labelY.Refresh()
	}

	backend := resolveBackend(snapshot.backend)
	if backend == BackendCPU {
		if !renderer.cpuConfigured || renderer.cpuRevision != snapshot.revision {
			renderer.rasterState.set(snapshot)
			renderer.cpuConfigured = true
			renderer.cpuRevision = snapshot.revision
			renderer.raster.Refresh()
		}
		showObject(renderer.raster, true)
		for _, shader := range renderer.shaders {
			showObject(shader, false)
		}
	} else {
		showObject(renderer.raster, false)
		updateShaders := !renderer.gpuConfigured || renderer.gpuRevision != snapshot.revision
		for index, shader := range renderer.shaders {
			visible := index < len(snapshot.series) && len(snapshot.series[index].Points) > 0
			showObject(shader, visible)
			if visible && updateShaders {
				configureSeriesShader(shader, snapshot.series[index], snapshot.view, seriesColor(index))
			}
		}
		if updateShaders {
			renderer.gpuConfigured = true
			renderer.gpuRevision = snapshot.revision
		}
	}

	renderer.refreshAxes(snapshot, colors)
	renderer.refreshLegend(snapshot, colors)
	renderer.refreshHover(snapshot, colors)
	renderer.layout(renderer.plot.Size(), snapshot)
	for _, object := range renderer.objects {
		switch object.(type) {
		case *canvas.Raster, *canvas.Shader:
			// Серии обновляются только при смене revision выше. Иначе один hover
			// заставлял бы CPU перерисовывать всё изображение, а GPU — заново
			// загружать текстуры.
			continue
		}
		canvas.Refresh(object)
	}
}

func (renderer *plotRenderer) layout(size fyne.Size, snapshot plotSnapshot) {
	if size.Width < plotMarginLeft+plotMarginRight+1 {
		size.Width = plotMarginLeft + plotMarginRight + 1
	}
	if size.Height < plotMarginTop+plotMarginBottom+1 {
		size.Height = plotMarginTop + plotMarginBottom + 1
	}
	plotPosition := fyne.NewPos(plotMarginLeft, plotMarginTop)
	plotSize := fyne.NewSize(size.Width-plotMarginLeft-plotMarginRight, size.Height-plotMarginTop-plotMarginBottom)
	renderer.background.Move(fyne.NewPos(0, 0))
	renderer.background.Resize(size)
	renderer.plotBackground.Move(plotPosition)
	renderer.plotBackground.Resize(plotSize)
	renderer.raster.Move(plotPosition)
	renderer.raster.Resize(plotSize)
	for _, shader := range renderer.shaders {
		shader.Move(plotPosition)
		shader.Resize(plotSize)
	}

	renderer.axisX.Position1 = fyne.NewPos(plotPosition.X, plotPosition.Y+plotSize.Height)
	renderer.axisX.Position2 = fyne.NewPos(plotPosition.X+plotSize.Width, plotPosition.Y+plotSize.Height)
	renderer.axisY.Position1 = plotPosition
	renderer.axisY.Position2 = fyne.NewPos(plotPosition.X, plotPosition.Y+plotSize.Height)

	renderer.layoutAxisX(snapshot, plotPosition, plotSize)
	renderer.layoutAxisY(snapshot, plotPosition, plotSize)
	renderer.labelX.Move(fyne.NewPos(plotPosition.X, size.Height-25))
	renderer.labelX.Resize(fyne.NewSize(plotSize.Width, 22))
	renderer.labelY.Move(fyne.NewPos(2, plotPosition.Y))
	renderer.labelY.Resize(fyne.NewSize(24, plotSize.Height))
	renderer.layoutLegend(snapshot, plotPosition, plotSize)
	renderer.layoutHover(snapshot, plotPosition, plotSize, size)
}

func (renderer *plotRenderer) refreshAxes(snapshot plotSnapshot, colors palette) {
	for _, line := range renderer.gridX {
		line.StrokeColor = colors.grid
	}
	for _, line := range renderer.gridY {
		line.StrokeColor = colors.grid
	}
	for _, label := range renderer.tickX {
		label.Color = colors.foreground
	}
	for _, label := range renderer.tickY {
		label.Color = colors.foreground
	}
	for index := 0; index < maxAxisTicks; index++ {
		xVisible := index < snapshot.axes.X.Ticks
		yVisible := index < snapshot.axes.Y.Ticks
		showObject(renderer.tickX[index], xVisible)
		showObject(renderer.gridX[index], xVisible && !snapshot.axes.HideGrid)
		showObject(renderer.tickY[index], yVisible)
		showObject(renderer.gridY[index], yVisible && !snapshot.axes.HideGrid)
	}
}

func (renderer *plotRenderer) layoutAxisX(snapshot plotSnapshot, position fyne.Position, size fyne.Size) {
	ticks := snapshot.axes.X.Ticks
	for index := 0; index < ticks; index++ {
		ratio := tickRatio(index, ticks)
		x := position.X + float32(ratio)*size.Width
		value := snapshot.view.xMin + ratio*(snapshot.view.xMax-snapshot.view.xMin)
		renderer.tickX[index].Text = formatAxisValue(snapshot.axes.X, value)
		renderer.tickX[index].Move(fyne.NewPos(x-45, position.Y+size.Height+7))
		renderer.tickX[index].Resize(fyne.NewSize(90, 18))
		renderer.gridX[index].Position1 = fyne.NewPos(x, position.Y)
		renderer.gridX[index].Position2 = fyne.NewPos(x, position.Y+size.Height)
	}
}

func (renderer *plotRenderer) layoutAxisY(snapshot plotSnapshot, position fyne.Position, size fyne.Size) {
	ticks := snapshot.axes.Y.Ticks
	for index := 0; index < ticks; index++ {
		ratio := tickRatio(index, ticks)
		y := position.Y + size.Height - float32(ratio)*size.Height
		value := snapshot.view.yMin + ratio*(snapshot.view.yMax-snapshot.view.yMin)
		renderer.tickY[index].Text = formatAxisValue(snapshot.axes.Y, value)
		renderer.tickY[index].Move(fyne.NewPos(29, y-9))
		renderer.tickY[index].Resize(fyne.NewSize(plotMarginLeft-35, 18))
		renderer.gridY[index].Position1 = fyne.NewPos(position.X, y)
		renderer.gridY[index].Position2 = fyne.NewPos(position.X+size.Width, y)
	}
}

func (renderer *plotRenderer) refreshLegend(snapshot plotSnapshot, colors palette) {
	visible := snapshot.legend && len(snapshot.series) > 0
	showObject(renderer.legendBox, visible)
	renderer.legendBox.FillColor = colors.legend
	for index := range renderer.legendTexts {
		itemVisible := visible && index < len(snapshot.series)
		showObject(renderer.legendTexts[index], itemVisible)
		showObject(renderer.legendSwatches[index], itemVisible)
		if itemVisible {
			renderer.legendTexts[index].Text = snapshot.series[index].Name
			renderer.legendTexts[index].Color = colors.foreground
			configured := snapshot.series[index].Color
			if configured == nil {
				configured = seriesColor(index)
			}
			renderer.legendSwatches[index].FillColor = configured
		}
	}
}

func (renderer *plotRenderer) layoutLegend(snapshot plotSnapshot, position fyne.Position, size fyne.Size) {
	if !snapshot.legend || len(snapshot.series) == 0 {
		return
	}
	boxWidth := float32(180)
	boxHeight := float32(12 + len(snapshot.series)*22)
	boxX := position.X + size.Width - boxWidth - 8
	boxY := position.Y + 8
	renderer.legendBox.Move(fyne.NewPos(boxX, boxY))
	renderer.legendBox.Resize(fyne.NewSize(boxWidth, boxHeight))
	for index := range snapshot.series {
		y := boxY + 8 + float32(index*22)
		renderer.legendSwatches[index].Move(fyne.NewPos(boxX+8, y+5))
		renderer.legendSwatches[index].Resize(fyne.NewSize(20, 4))
		renderer.legendTexts[index].Move(fyne.NewPos(boxX+35, y))
		renderer.legendTexts[index].Resize(fyne.NewSize(boxWidth-43, 18))
	}
}

func (renderer *plotRenderer) refreshHover(snapshot plotSnapshot, colors palette) {
	visible := snapshot.hover != nil
	showObject(renderer.hoverMarker, visible)
	showObject(renderer.hoverBox, visible)
	showObject(renderer.hoverText, visible)
	if !visible {
		return
	}
	renderer.hoverMarker.FillColor = color.Transparent
	renderer.hoverMarker.StrokeColor = colors.marker
	renderer.hoverBox.FillColor = colors.tooltip
	renderer.hoverText.Color = colors.tooltipText
	renderer.hoverText.Text = fmt.Sprintf("%s   X: %s   Y: %s",
		snapshot.hover.seriesName,
		formatAxisValue(snapshot.axes.X, snapshot.hover.point.X),
		formatAxisValue(snapshot.axes.Y, snapshot.hover.point.Y),
	)
}

func (renderer *plotRenderer) layoutHover(snapshot plotSnapshot, position fyne.Position, plotSize fyne.Size, widgetSize fyne.Size) {
	if snapshot.hover == nil {
		return
	}
	xRatio := (snapshot.hover.point.X - snapshot.view.xMin) / (snapshot.view.xMax - snapshot.view.xMin)
	yRatio := (snapshot.hover.point.Y - snapshot.view.yMin) / (snapshot.view.yMax - snapshot.view.yMin)
	x := position.X + float32(xRatio)*plotSize.Width
	y := position.Y + plotSize.Height - float32(yRatio)*plotSize.Height
	renderer.hoverMarker.Move(fyne.NewPos(x-5, y-5))
	renderer.hoverMarker.Resize(fyne.NewSize(10, 10))
	textWidth := float32(math.Max(130, float64(len([]rune(renderer.hoverText.Text))*7+16)))
	textHeight := float32(28)
	boxX := snapshot.hover.position.X + 14
	boxY := snapshot.hover.position.Y - textHeight - 8
	if boxX+textWidth > widgetSize.Width-4 {
		boxX = snapshot.hover.position.X - textWidth - 14
	}
	if boxY < 4 {
		boxY = snapshot.hover.position.Y + 14
	}
	renderer.hoverBox.Move(fyne.NewPos(boxX, boxY))
	renderer.hoverBox.Resize(fyne.NewSize(textWidth, textHeight))
	renderer.hoverText.Move(fyne.NewPos(boxX+8, boxY+5))
	renderer.hoverText.Resize(fyne.NewSize(textWidth-16, 18))
}

func tickRatio(index int, ticks int) float64 {
	if ticks < 2 {
		return 0
	}
	return float64(index) / float64(ticks-1)
}

func formatAxisValue(axis AxisConfig, value float64) (formatted string) {
	if axis.Formatter == nil {
		return strconv.FormatFloat(value, 'g', 5, 64)
	}
	defer func() {
		if recover() != nil {
			formatted = strconv.FormatFloat(value, 'g', 5, 64)
		}
	}()
	return axis.Formatter(value)
}

func resolveBackend(configured RenderBackend) RenderBackend {
	if configured != BackendAuto {
		return configured
	}
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return BackendCPU
	}
	driverName := strings.ToLower(reflect.TypeOf(app.Driver()).String())
	if strings.Contains(driverName, "test") || strings.Contains(driverName, "software") {
		return BackendCPU
	}
	return BackendGPU
}

func showObject(object fyne.CanvasObject, visible bool) {
	if visible && !object.Visible() {
		object.Show()
	}
	if !visible && object.Visible() {
		object.Hide()
	}
}

func themePalette(theme ThemeVariant) palette {
	if theme == ThemeLight {
		return palette{
			background:     color.NRGBA{R: 245, G: 247, B: 250, A: 255},
			plotBackground: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
			foreground:     color.NRGBA{R: 35, G: 39, B: 47, A: 255},
			grid:           color.NRGBA{R: 210, G: 216, B: 225, A: 190},
			legend:         color.NRGBA{R: 238, G: 241, B: 246, A: 235},
			tooltip:        color.NRGBA{R: 35, G: 39, B: 47, A: 245},
			tooltipText:    color.White,
			marker:         color.NRGBA{R: 35, G: 39, B: 47, A: 255},
		}
	}
	return palette{
		background:     color.NRGBA{R: 20, G: 23, B: 29, A: 255},
		plotBackground: color.NRGBA{R: 27, G: 31, B: 39, A: 255},
		foreground:     color.NRGBA{R: 232, G: 235, B: 242, A: 255},
		grid:           color.NRGBA{R: 75, G: 82, B: 96, A: 180},
		legend:         color.NRGBA{R: 38, G: 43, B: 53, A: 235},
		tooltip:        color.NRGBA{R: 238, G: 241, B: 247, A: 245},
		tooltipText:    color.NRGBA{R: 24, G: 27, B: 33, A: 255},
		marker:         color.White,
	}
}

func seriesColor(index int) color.Color {
	colors := []color.NRGBA{
		{R: 68, G: 138, B: 255, A: 255},
		{R: 255, G: 105, B: 97, A: 255},
		{R: 71, G: 201, B: 142, A: 255},
		{R: 245, G: 190, B: 66, A: 255},
		{R: 177, G: 113, B: 255, A: 255},
		{R: 45, G: 202, B: 211, A: 255},
		{R: 255, G: 134, B: 205, A: 255},
		{R: 167, G: 186, B: 94, A: 255},
	}
	return colors[index%len(colors)]
}
