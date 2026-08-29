package plot

import (
	"image"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2/canvas"
)

type cpuFrame struct {
	series []Series
	view   axisRange
}

type cpuRasterState struct {
	mu    sync.RWMutex
	frame cpuFrame
}

func newCPURaster() (*canvas.Raster, *cpuRasterState) {
	state := &cpuRasterState{}
	raster := canvas.NewRaster(state.render)
	return raster, state
}

func (state *cpuRasterState) set(snapshot plotSnapshot) {
	frame := cpuFrame{view: snapshot.view, series: make([]Series, len(snapshot.series))}
	for index, series := range snapshot.series {
		frame.series[index] = series
		frame.series[index].Points = normalizedSeriesPoints(series.Points, snapshot.view)
	}
	state.mu.Lock()
	state.frame = frame
	state.mu.Unlock()
}

func (state *cpuRasterState) render(width int, height int) image.Image {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	state.mu.RLock()
	frame := state.frame
	state.mu.RUnlock()
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for index, series := range frame.series {
		fallback := seriesColor(index)
		configured := series.Color
		if configured == nil {
			configured = fallback
		}
		value := color.NRGBAModel.Convert(configured).(color.NRGBA)
		drawCPUSeries(result, series, value, frame.view)
	}
	return result
}

func drawCPUSeries(target *image.RGBA, series Series, value color.NRGBA, view axisRange) {
	if len(series.Points) == 0 {
		return
	}
	width := float64(target.Bounds().Dx() - 1)
	height := float64(target.Bounds().Dy() - 1)
	toPixel := func(point Point) (float64, float64) {
		return point.X * width, (1 - point.Y) * height
	}
	lineRadius := math.Max(0.5, float64(series.LineWidth)*0.5)
	pointRadius := math.Max(0.5, float64(series.PointRadius))

	switch series.Mode {
	case DrawLine:
		if len(series.Points) == 1 {
			x, y := toPixel(series.Points[0])
			drawDisk(target, x, y, lineRadius, value)
			return
		}
		for index := 1; index < len(series.Points); index++ {
			x1, y1 := toPixel(series.Points[index-1])
			x2, y2 := toPixel(series.Points[index])
			drawLine(target, x1, y1, x2, y2, lineRadius, value)
		}
	case DrawPoints:
		for _, point := range series.Points {
			x, y := toPixel(point)
			drawDisk(target, x, y, pointRadius, value)
		}
	case DrawPopsicle:
		base := clampCoordinate((0 - view.yMin) / (view.yMax - view.yMin))
		base = math.Max(0, math.Min(1, base))
		for _, point := range series.Points {
			x, y := toPixel(point)
			_, baseY := toPixel(Point{X: point.X, Y: base})
			drawLine(target, x, baseY, x, y, lineRadius, value)
			drawDisk(target, x, y, pointRadius, value)
		}
	}
}

func drawLine(target *image.RGBA, x1 float64, y1 float64, x2 float64, y2 float64, radius float64, value color.NRGBA) {
	distance := math.Hypot(x2-x1, y2-y1)
	steps := int(math.Ceil(distance))
	if steps < 1 {
		drawDisk(target, x1, y1, radius, value)
		return
	}
	for step := 0; step <= steps; step++ {
		ratio := float64(step) / float64(steps)
		drawDisk(target, x1+(x2-x1)*ratio, y1+(y2-y1)*ratio, radius, value)
	}
}

func drawDisk(target *image.RGBA, centerX float64, centerY float64, radius float64, value color.NRGBA) {
	minimumX := int(math.Floor(centerX - radius - 1))
	maximumX := int(math.Ceil(centerX + radius + 1))
	minimumY := int(math.Floor(centerY - radius - 1))
	maximumY := int(math.Ceil(centerY + radius + 1))
	for y := minimumY; y <= maximumY; y++ {
		for x := minimumX; x <= maximumX; x++ {
			distance := math.Hypot(float64(x)+0.5-centerX, float64(y)+0.5-centerY)
			coverage := math.Max(0, math.Min(1, radius+0.75-distance))
			if coverage == 0 {
				continue
			}
			alpha := uint8(math.Round(float64(value.A) * coverage))
			blendPixel(target, x, y, color.NRGBA{R: value.R, G: value.G, B: value.B, A: alpha})
		}
	}
}

func blendPixel(target *image.RGBA, x int, y int, source color.NRGBA) {
	if !image.Pt(x, y).In(target.Bounds()) || source.A == 0 {
		return
	}
	offset := target.PixOffset(x, y)
	destinationAlpha := float64(target.Pix[offset+3]) / 255
	sourceAlpha := float64(source.A) / 255
	outputAlpha := sourceAlpha + destinationAlpha*(1-sourceAlpha)
	if outputAlpha == 0 {
		return
	}
	blend := func(sourceValue uint8, destinationValue uint8) uint8 {
		value := (float64(sourceValue)*sourceAlpha + float64(destinationValue)*destinationAlpha*(1-sourceAlpha)) / outputAlpha
		return uint8(math.Round(value))
	}
	target.Pix[offset] = blend(source.R, target.Pix[offset])
	target.Pix[offset+1] = blend(source.G, target.Pix[offset+1])
	target.Pix[offset+2] = blend(source.B, target.Pix[offset+2])
	target.Pix[offset+3] = uint8(math.Round(outputAlpha * 255))
}
