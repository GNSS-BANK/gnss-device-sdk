package plot

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Scrolled масштабирует график относительно положения указателя.
func (plot *plotWidget) Scrolled(event *fyne.ScrollEvent) {
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
func (plot *plotWidget) Dragged(event *fyne.DragEvent) {
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
