package plot

import (
	"math"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Scrolled масштабирует график относительно положения указателя.
func (plot *plotWidget) Scrolled(event *fyne.ScrollEvent) {
	if plot == nil || event == nil || event.Scrolled.DY == 0 {
		return
	}
	size := plot.Size()
	metrics := newPlotLayoutMetrics(plot.fontSize)
	plotWidth := size.Width - metrics.marginLeft - metrics.marginRight
	plotHeight := size.Height - metrics.marginTop - metrics.marginBottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
	}
	position := event.Position
	if position.X < metrics.marginLeft || position.X > metrics.marginLeft+plotWidth || position.Y < metrics.marginTop || position.Y > metrics.marginTop+plotHeight {
		return
	}
	factor := 1.25
	if event.Scrolled.DY > 0 {
		factor = 0.8
	}
	xRatio := float64((position.X - metrics.marginLeft) / plotWidth)
	yRatio := float64((metrics.marginTop + plotHeight - position.Y) / plotHeight)
	_ = plot.zoomAt(factor, xRatio, yRatio)
}

// Dragged перемещает видимую область вслед за указателем. После ручного
// перемещения автоматический диапазон возобновляется через ResetZoom.
func (plot *plotWidget) Dragged(event *fyne.DragEvent) {
	if plot == nil || event == nil || (event.Dragged.DX == 0 && event.Dragged.DY == 0) {
		return
	}
	size := plot.Size()
	metrics := newPlotLayoutMetrics(plot.fontSize)
	plotWidth := size.Width - metrics.marginLeft - metrics.marginRight
	plotHeight := size.Height - metrics.marginTop - metrics.marginBottom
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
func (plot *plotWidget) DragEnd() {}

// MouseIn обновляет подсказку при входе указателя в область виджета.
func (plot *plotWidget) MouseIn(event *desktop.MouseEvent) {
	plot.MouseMoved(event)
}

// MouseMoved находит ближайшую точку и показывает её точное значение.
func (plot *plotWidget) MouseMoved(event *desktop.MouseEvent) {
	if plot == nil || event == nil {
		return
	}
	snapshot := plot.snapshot()
	metrics := newPlotLayoutMetrics(snapshot.fontSize)
	size := plot.Size()
	plotWidth := size.Width - metrics.marginLeft - metrics.marginRight
	plotHeight := size.Height - metrics.marginTop - metrics.marginBottom
	position := event.Position
	if plotWidth <= 0 || plotHeight <= 0 || position.X < metrics.marginLeft || position.X > metrics.marginLeft+plotWidth || position.Y < metrics.marginTop || position.Y > metrics.marginTop+plotHeight {
		plot.clearHover()
		return
	}

	bestDistance := math.Inf(1)
	var best *hoverState
	hitRadius := float64(metrics.value(14))
	cursorXRatio := float64((position.X - metrics.marginLeft) / plotWidth)
	cursorX := snapshot.view.xMin + cursorXRatio*(snapshot.view.xMax-snapshot.view.xMin)
	xTolerance := hitRadius / float64(plotWidth) * (snapshot.view.xMax - snapshot.view.xMin)
	for _, series := range snapshot.series {
		start := sort.Search(len(series.Points), func(index int) bool {
			return series.Points[index].X >= cursorX-xTolerance
		})
		end := sort.Search(len(series.Points), func(index int) bool {
			return series.Points[index].X > cursorX+xTolerance
		})
		for _, point := range series.Points[start:end] {
			xRatio := (point.X - snapshot.view.xMin) / (snapshot.view.xMax - snapshot.view.xMin)
			yRatio := (point.Y - snapshot.view.yMin) / (snapshot.view.yMax - snapshot.view.yMin)
			if xRatio < 0 || xRatio > 1 || yRatio < 0 || yRatio > 1 {
				continue
			}
			x := metrics.marginLeft + float32(xRatio)*plotWidth
			y := metrics.marginTop + plotHeight - float32(yRatio)*plotHeight
			distance := math.Hypot(float64(position.X-x), float64(position.Y-y))
			if distance < bestDistance {
				bestDistance = distance
				best = &hoverState{seriesID: series.ID, seriesName: series.Name, point: point, position: position}
			}
		}
	}
	if bestDistance > hitRadius {
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
func (plot *plotWidget) MouseOut() {
	plot.clearHover()
}

func (plot *plotWidget) clearHover() {
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

var _ fyne.Scrollable = (*plotWidget)(nil)
var _ fyne.Draggable = (*plotWidget)(nil)
var _ desktop.Hoverable = (*plotWidget)(nil)
