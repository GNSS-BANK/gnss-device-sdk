package plot

import (
	"errors"
	"image"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Scrolled масштабирует график относительно положения указателя.
func (plot *Plot) Scrolled(event *fyne.ScrollEvent) {
	if plot == nil || event == nil || event.Scrolled.DY == 0 {
		return
	}
	size := plot.Size()
	plotWidth := size.Width - plotMarginLeft - plotMarginRight
	plotHeight := size.Height - plotMarginTop - plotMarginBottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
	}
	position := event.Position
	if position.X < plotMarginLeft || position.X > plotMarginLeft+plotWidth || position.Y < plotMarginTop || position.Y > plotMarginTop+plotHeight {
		return
	}
	factor := 1.25
	if event.Scrolled.DY > 0 {
		factor = 0.8
	}
	xRatio := float64((position.X - plotMarginLeft) / plotWidth)
	yRatio := float64((plotMarginTop + plotHeight - position.Y) / plotHeight)
	_ = plot.zoomAt(factor, xRatio, yRatio)
}

// Dragged перемещает видимую область вслед за указателем. После ручного
// перемещения автоматический диапазон возобновляется через ResetZoom.
func (plot *Plot) Dragged(event *fyne.DragEvent) {
	if plot == nil || event == nil || (event.Dragged.DX == 0 && event.Dragged.DY == 0) {
		return
	}
	size := plot.Size()
	plotWidth := size.Width - plotMarginLeft - plotMarginRight
	plotHeight := size.Height - plotMarginTop - plotMarginBottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
	}

	plot.mu.Lock()
	current := plot.currentRangeLocked()
	xShift := -float64(event.Dragged.DX/plotWidth) * (current.xMax - current.xMin)
	yShift := float64(event.Dragged.DY/plotHeight) * (current.yMax - current.yMin)
	plot.view = &axisRange{
		xMin: current.xMin + xShift,
		xMax: current.xMax + xShift,
		yMin: current.yMin + yShift,
		yMax: current.yMax + yShift,
	}
	plot.hover = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
}

// DragEnd завершает жест перемещения. Состояние viewport уже обновлено в Dragged.
func (plot *Plot) DragEnd() {}

// MouseIn обновляет подсказку при входе указателя в область виджета.
func (plot *Plot) MouseIn(event *desktop.MouseEvent) {
	plot.MouseMoved(event)
}

// MouseMoved находит ближайшую точку и показывает её точное значение.
func (plot *Plot) MouseMoved(event *desktop.MouseEvent) {
	if plot == nil || event == nil {
		return
	}
	snapshot := plot.snapshot()
	size := plot.Size()
	plotWidth := size.Width - plotMarginLeft - plotMarginRight
	plotHeight := size.Height - plotMarginTop - plotMarginBottom
	position := event.Position
	if plotWidth <= 0 || plotHeight <= 0 || position.X < plotMarginLeft || position.X > plotMarginLeft+plotWidth || position.Y < plotMarginTop || position.Y > plotMarginTop+plotHeight {
		plot.clearHover()
		return
	}

	bestDistance := math.Inf(1)
	var best *hoverState
	for _, series := range snapshot.series {
		for _, point := range series.Points {
			xRatio := (point.X - snapshot.view.xMin) / (snapshot.view.xMax - snapshot.view.xMin)
			yRatio := (point.Y - snapshot.view.yMin) / (snapshot.view.yMax - snapshot.view.yMin)
			if xRatio < 0 || xRatio > 1 || yRatio < 0 || yRatio > 1 {
				continue
			}
			x := plotMarginLeft + float32(xRatio)*plotWidth
			y := plotMarginTop + plotHeight - float32(yRatio)*plotHeight
			distance := math.Hypot(float64(position.X-x), float64(position.Y-y))
			if distance < bestDistance {
				bestDistance = distance
				best = &hoverState{seriesID: series.ID, seriesName: series.Name, point: point, position: position}
			}
		}
	}
	if bestDistance > 14 {
		best = nil
	}

	plot.mu.Lock()
	changed := !sameHover(plot.hover, best)
	plot.hover = best
	plot.mu.Unlock()
	if changed {
		plot.requestRefresh()
	}
}

// MouseOut скрывает подсказку.
func (plot *Plot) MouseOut() {
	plot.clearHover()
}

func (plot *Plot) clearHover() {
	plot.mu.Lock()
	changed := plot.hover != nil
	plot.hover = nil
	plot.mu.Unlock()
	if changed {
		plot.requestRefresh()
	}
}

func sameHover(left *hoverState, right *hoverState) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.seriesID == right.seriesID && left.point == right.point && left.position == right.position
}

// Capture возвращает текущее изображение Fyne canvas. Для изображения только
// графика передайте canvas окна, где Plot установлен корневым content.
// Метод следует вызывать из UI-горутины Fyne.
func (plot *Plot) Capture(canvas fyne.Canvas) (image.Image, error) {
	if plot == nil {
		return nil, errors.New("plot is nil")
	}
	if canvas == nil {
		return nil, errors.New("fyne canvas is nil")
	}
	plot.Refresh()
	result := canvas.Capture()
	if result == nil || result.Bounds().Empty() {
		return nil, errors.New("fyne canvas returned an empty image")
	}
	return result, nil
}

var _ fyne.Scrollable = (*Plot)(nil)
var _ fyne.Draggable = (*Plot)(nil)
var _ desktop.Hoverable = (*Plot)(nil)
