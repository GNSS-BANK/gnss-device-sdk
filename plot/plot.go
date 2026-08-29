package plot

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Chart — публичный интерфейс интерактивного графика.
type Chart interface {
	SetSeries([]Series) error
	AddSeries(Series) error
	RemoveSeries(seriesID string) bool
	Append(seriesID string, points ...Point) error
	Clear()
	ClearSeries(seriesID string) bool
	Pause()
	Resume()
	Paused() bool
	Zoom(factor float64) error
	ResetZoom()
	SetLegendVisible(visible bool)
	SetAxes(AxesConfig) error
	SetTheme(ThemeVariant) error
	SetBackend(RenderBackend) error
	Backend() RenderBackend
}

// Option настраивает создаваемый график.
type Option func(*plotOptions) error

type plotOptions struct {
	maxPoints int
	maxSeries int
	minSize   fyne.Size
	backend   RenderBackend
	theme     ThemeVariant
	font      FontFamily
	fontSize  float32
}

// WithMaxPoints задаёт размер скользящего окна каждой потоковой серии.
func WithMaxPoints(maxPoints int) Option {
	return func(options *plotOptions) error {
		if maxPoints < 1 || maxPoints > MaxGPUPoints {
			return fmt.Errorf("plot max points must be between 1 and %d", MaxGPUPoints)
		}
		options.maxPoints = maxPoints
		return nil
	}
}

// WithMaxSeries задаёт максимальное число одновременно отображаемых серий.
func WithMaxSeries(maxSeries int) Option {
	return func(options *plotOptions) error {
		if maxSeries < 1 || maxSeries > 16 {
			return errors.New("plot max series must be between 1 and 16")
		}
		options.maxSeries = maxSeries
		return nil
	}
}

// WithMinSize задаёт минимальный размер графика.
func WithMinSize(size Size) Option {
	return func(options *plotOptions) error {
		if size.Width <= 0 || size.Height <= 0 {
			return errors.New("plot minimum size must be positive")
		}
		options.minSize = fyne.NewSize(size.Width, size.Height)
		return nil
	}
}

// WithBackend задаёт автоматический, GPU- или CPU-renderer.
func WithBackend(backend RenderBackend) Option {
	return func(options *plotOptions) error {
		if backend > BackendCPU {
			return errors.New("unknown plot render backend")
		}
		options.backend = backend
		return nil
	}
}

// WithTheme задаёт начальную светлую или тёмную тему графика.
func WithTheme(theme ThemeVariant) Option {
	return func(options *plotOptions) error {
		if theme != ThemeDark && theme != ThemeLight {
			return errors.New("unknown plot theme")
		}
		options.theme = theme
		return nil
	}
}

// WithFont задаёт шрифт графика и стандартной панели управления.
func WithFont(font FontFamily) Option {
	return func(options *plotOptions) error {
		if err := validateFontFamily(font); err != nil {
			return err
		}
		options.font = font
		return nil
	}
}

// WithFontSize задаёт базовый размер шрифта в логических пикселях. Остальные
// текстовые элементы и поля графика масштабируются пропорционально.
func WithFontSize(size float32) Option {
	return func(options *plotOptions) error {
		if err := validateFontSize(size); err != nil {
			return err
		}
		options.fontSize = size
		return nil
	}
}

type axisRange struct {
	xMin float64
	xMax float64
	yMin float64
	yMax float64
}

type hoverState struct {
	seriesID   string
	seriesName string
	point      Point
	position   fyne.Position
}

type plotSnapshot struct {
	series      []Series
	axes        AxesConfig
	view        axisRange
	revision    uint64
	theme       ThemeVariant
	font        FontFamily
	fontSize    float32
	backend     RenderBackend
	legend      bool
	hover       *hoverState
	maxSeries   int
	minimumSize fyne.Size
}

// plotWidget — внутренняя реализация Chart и Fyne-виджет.
// Конкретный тип не экспортируется, чтобы Fyne не попадал в публичный API.
type plotWidget struct {
	widget.BaseWidget

	mu            sync.RWMutex
	series        []Series
	axes          AxesConfig
	theme         ThemeVariant
	font          FontFamily
	fontSize      float32
	backend       RenderBackend
	legendVisible bool
	paused        bool
	frozen        map[string][]Point
	view          *axisRange
	hover         *hoverState
	maxPoints     int
	maxSeries     int
	minimumSize   fyne.Size
	revision      uint64
	refreshQueued atomic.Bool
}

// New создаёт пустой интерактивный график. BackendAuto используется по умолчанию.
func New(options ...Option) (Chart, error) {
	return newPlot(options...)
}

func newPlot(options ...Option) (*plotWidget, error) {
	settings := plotOptions{
		maxPoints: 2048,
		maxSeries: 8,
		minSize:   fyne.NewSize(640, 360),
		theme:     ThemeDark,
		font:      FontDefault,
		fontSize:  defaultFontSize,
	}
	for _, option := range options {
		if option != nil {
			if err := option(&settings); err != nil {
				return nil, err
			}
		}
	}

	plot := &plotWidget{
		axes: AxesConfig{
			X: AxisConfig{Label: "X", Ticks: 5},
			Y: AxisConfig{Label: "Y", Ticks: 5},
		},
		theme:         settings.theme,
		font:          settings.font,
		fontSize:      settings.fontSize,
		backend:       settings.backend,
		legendVisible: true,
		maxPoints:     settings.maxPoints,
		maxSeries:     settings.maxSeries,
		minimumSize:   settings.minSize,
		revision:      1,
	}
	plot.ExtendBaseWidget(plot)
	return plot, nil
}

// SetSeries полностью заменяет набор серий.
func (plot *plotWidget) SetSeries(series []Series) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if len(series) > plot.maxSeries {
		return fmt.Errorf("plot supports at most %d series", plot.maxSeries)
	}

	normalized := make([]Series, 0, len(series))
	ids := make(map[string]struct{}, len(series))
	for _, item := range series {
		current, err := plot.normalizeSeries(item)
		if err != nil {
			return err
		}
		if _, duplicate := ids[current.ID]; duplicate {
			return fmt.Errorf("plot series %q is configured more than once", current.ID)
		}
		ids[current.ID] = struct{}{}
		normalized = append(normalized, current)
	}

	plot.mu.Lock()
	plot.series = normalized
	plot.frozen = nil
	if plot.paused {
		plot.frozen = make(map[string][]Point, len(normalized))
		for _, series := range normalized {
			plot.frozen[series.ID] = slices.Clone(series.Points)
		}
	}
	plot.view = nil
	plot.hover = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// AddSeries добавляет одну серию.
func (plot *plotWidget) AddSeries(series Series) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	current, err := plot.normalizeSeries(series)
	if err != nil {
		return err
	}

	plot.mu.Lock()
	if len(plot.series) >= plot.maxSeries {
		plot.mu.Unlock()
		return fmt.Errorf("plot supports at most %d series", plot.maxSeries)
	}
	if plot.seriesIndexLocked(current.ID) >= 0 {
		plot.mu.Unlock()
		return fmt.Errorf("plot series %q already exists", current.ID)
	}
	plot.series = append(plot.series, current)
	if plot.paused {
		if plot.frozen == nil {
			plot.frozen = make(map[string][]Point)
		}
		plot.frozen[current.ID] = slices.Clone(current.Points)
	}
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// RemoveSeries удаляет серию по идентификатору.
func (plot *plotWidget) RemoveSeries(seriesID string) bool {
	if plot == nil {
		return false
	}
	seriesID = strings.TrimSpace(seriesID)
	plot.mu.Lock()
	index := plot.seriesIndexLocked(seriesID)
	if index < 0 {
		plot.mu.Unlock()
		return false
	}
	plot.series = append(plot.series[:index], plot.series[index+1:]...)
	delete(plot.frozen, seriesID)
	plot.hover = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return true
}

// Append добавляет точки в потоковую серию. X должен монотонно не убывать.
func (plot *plotWidget) Append(seriesID string, points ...Point) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if len(points) == 0 {
		return nil
	}
	for _, point := range points {
		if err := validatePoint(point); err != nil {
			return err
		}
	}
	for index := 1; index < len(points); index++ {
		if points[index].X < points[index-1].X {
			return errors.New("streaming plot points must have monotonically increasing X")
		}
	}

	seriesID = strings.TrimSpace(seriesID)
	plot.mu.Lock()
	index := plot.seriesIndexLocked(seriesID)
	if index < 0 {
		plot.mu.Unlock()
		return fmt.Errorf("plot series %q not found", seriesID)
	}
	current := &plot.series[index]
	if len(current.Points) > 0 && points[0].X < current.Points[len(current.Points)-1].X {
		plot.mu.Unlock()
		return fmt.Errorf("streaming plot series %q requires monotonically increasing X", seriesID)
	}
	current.Points = append(current.Points, points...)
	if overflow := len(current.Points) - plot.maxPoints; overflow > 0 {
		copy(current.Points, current.Points[overflow:])
		current.Points = current.Points[:plot.maxPoints]
	}
	paused := plot.paused
	if !paused {
		plot.revision++
	}
	plot.mu.Unlock()
	if !paused {
		plot.requestRefresh()
	}
	return nil
}

// Clear очищает данные всех серий, сохраняя их настройки.
func (plot *plotWidget) Clear() {
	if plot == nil {
		return
	}
	plot.mu.Lock()
	for index := range plot.series {
		plot.series[index].Points = nil
	}
	plot.frozen = nil
	if plot.paused {
		plot.frozen = make(map[string][]Point, len(plot.series))
	}
	plot.view = nil
	plot.hover = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
}

// ClearSeries очищает данные одной серии.
func (plot *plotWidget) ClearSeries(seriesID string) bool {
	if plot == nil {
		return false
	}
	seriesID = strings.TrimSpace(seriesID)
	plot.mu.Lock()
	index := plot.seriesIndexLocked(seriesID)
	if index < 0 {
		plot.mu.Unlock()
		return false
	}
	plot.series[index].Points = nil
	if plot.paused {
		plot.frozen[seriesID] = nil
	}
	plot.hover = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return true
}

// Pause замораживает отображаемые данные. Append продолжает принимать точки.
func (plot *plotWidget) Pause() {
	if plot == nil {
		return
	}
	plot.mu.Lock()
	if plot.paused {
		plot.mu.Unlock()
		return
	}
	plot.paused = true
	plot.frozen = make(map[string][]Point, len(plot.series))
	for _, series := range plot.series {
		plot.frozen[series.ID] = slices.Clone(series.Points)
	}
	plot.mu.Unlock()
	plot.requestRefresh()
}

// Resume показывает актуальные накопленные данные и продолжает обновления.
func (plot *plotWidget) Resume() {
	if plot == nil {
		return
	}
	plot.mu.Lock()
	if !plot.paused {
		plot.mu.Unlock()
		return
	}
	plot.paused = false
	plot.frozen = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
}

// Paused сообщает, заморожено ли отображение.
func (plot *plotWidget) Paused() bool {
	if plot == nil {
		return false
	}
	plot.mu.RLock()
	defer plot.mu.RUnlock()
	return plot.paused
}

// Zoom изменяет масштаб относительно центра. factor < 1 приближает, factor > 1 отдаляет.
func (plot *plotWidget) Zoom(factor float64) error {
	return plot.zoomAt(factor, 0.5, 0.5)
}

func (plot *plotWidget) zoomAt(factor float64, xRatio float64, yRatio float64) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if math.IsNaN(factor) || math.IsInf(factor, 0) || factor <= 0 {
		return errors.New("plot zoom factor must be finite and > 0")
	}
	factor = math.Max(0.01, math.Min(100, factor))
	xRatio = math.Max(0, math.Min(1, xRatio))
	yRatio = math.Max(0, math.Min(1, yRatio))

	plot.mu.Lock()
	current := plot.currentRangeLocked()
	xCenter := current.xMin + xRatio*(current.xMax-current.xMin)
	yCenter := current.yMin + yRatio*(current.yMax-current.yMin)
	xWidth := (current.xMax - current.xMin) * factor
	yWidth := (current.yMax - current.yMin) * factor
	plot.view = &axisRange{
		xMin: xCenter - xRatio*xWidth,
		xMax: xCenter + (1-xRatio)*xWidth,
		yMin: yCenter - yRatio*yWidth,
		yMax: yCenter + (1-yRatio)*yWidth,
	}
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// ResetZoom возвращает границы из AxesConfig или автоматический диапазон.
func (plot *plotWidget) ResetZoom() {
	if plot == nil {
		return
	}
	plot.mu.Lock()
	plot.view = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
}

// SetLegendVisible показывает или скрывает легенду.
func (plot *plotWidget) SetLegendVisible(visible bool) {
	if plot == nil {
		return
	}
	plot.mu.Lock()
	plot.legendVisible = visible
	plot.mu.Unlock()
	plot.requestRefresh()
}

// SetAxes применяет настройки осей и сбрасывает текущий zoom.
func (plot *plotWidget) SetAxes(axes AxesConfig) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if err := validateAxes(axes); err != nil {
		return err
	}
	if axes.X.Ticks == 0 {
		axes.X.Ticks = 5
	}
	if axes.Y.Ticks == 0 {
		axes.Y.Ticks = 5
	}
	plot.mu.Lock()
	plot.axes = axes
	plot.view = nil
	plot.revision++
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// SetTheme переключает светлую или тёмную тему самого графика.
func (plot *plotWidget) SetTheme(theme ThemeVariant) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if theme != ThemeDark && theme != ThemeLight {
		return errors.New("unknown plot theme")
	}
	plot.mu.Lock()
	plot.theme = theme
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// SetBackend переключает GPU/CPU renderer без изменения данных и масштаба.
func (plot *plotWidget) SetBackend(backend RenderBackend) error {
	if plot == nil {
		return errors.New("plot is nil")
	}
	if backend > BackendCPU {
		return errors.New("unknown plot render backend")
	}
	plot.mu.Lock()
	plot.backend = backend
	plot.mu.Unlock()
	plot.requestRefresh()
	return nil
}

// Backend возвращает выбранный режим renderer. BackendAuto разрешается в
// конкретный GPU/CPU backend во время Refresh.
func (plot *plotWidget) Backend() RenderBackend {
	if plot == nil {
		return BackendAuto
	}
	plot.mu.RLock()
	defer plot.mu.RUnlock()
	return plot.backend
}

func (plot *plotWidget) normalizeSeries(series Series) (Series, error) {
	series.ID = strings.TrimSpace(series.ID)
	series.Name = strings.TrimSpace(series.Name)
	if series.ID == "" {
		return Series{}, errors.New("plot series ID is required")
	}
	if series.Name == "" {
		series.Name = series.ID
	}
	if series.Mode > DrawPopsicle {
		return Series{}, fmt.Errorf("plot series %q has unknown draw mode", series.ID)
	}
	if math.IsNaN(float64(series.LineWidth)) || math.IsInf(float64(series.LineWidth), 0) || series.LineWidth < 0 {
		return Series{}, fmt.Errorf("plot series %q line width must be finite and >= 0", series.ID)
	}
	if math.IsNaN(float64(series.PointRadius)) || math.IsInf(float64(series.PointRadius), 0) || series.PointRadius < 0 {
		return Series{}, fmt.Errorf("plot series %q point radius must be finite and >= 0", series.ID)
	}
	for _, point := range series.Points {
		if err := validatePoint(point); err != nil {
			return Series{}, fmt.Errorf("plot series %q: %w", series.ID, err)
		}
	}
	series.Points = slices.Clone(series.Points)
	sort.SliceStable(series.Points, func(left int, right int) bool {
		return series.Points[left].X < series.Points[right].X
	})
	if overflow := len(series.Points) - plot.maxPoints; overflow > 0 {
		series.Points = slices.Clone(series.Points[overflow:])
	}
	if series.LineWidth == 0 {
		series.LineWidth = 2
	}
	if series.PointRadius == 0 {
		series.PointRadius = 4
	}
	return series, nil
}

func (plot *plotWidget) seriesIndexLocked(seriesID string) int {
	for index := range plot.series {
		if plot.series[index].ID == seriesID {
			return index
		}
	}
	return -1
}

func (plot *plotWidget) requestRefresh() {
	if plot == nil || !plot.refreshQueued.CompareAndSwap(false, true) {
		return
	}
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		plot.refreshQueued.Store(false)
		plot.Refresh()
		return
	}
	fyne.Do(func() {
		plot.refreshQueued.Store(false)
		plot.Refresh()
	})
}

func (plot *plotWidget) snapshot() plotSnapshot {
	plot.mu.RLock()
	defer plot.mu.RUnlock()

	series := make([]Series, len(plot.series))
	for index, item := range plot.series {
		series[index] = item
		if plot.paused {
			series[index].Points = slices.Clone(plot.frozen[item.ID])
		} else {
			series[index].Points = slices.Clone(item.Points)
		}
	}
	var hover *hoverState
	if plot.hover != nil {
		copy := *plot.hover
		hover = &copy
	}
	return plotSnapshot{
		series:      series,
		axes:        plot.axes,
		view:        plot.currentRangeLocked(),
		revision:    plot.revision,
		theme:       plot.theme,
		font:        plot.font,
		fontSize:    plot.fontSize,
		backend:     plot.backend,
		legend:      plot.legendVisible,
		hover:       hover,
		maxSeries:   plot.maxSeries,
		minimumSize: plot.minimumSize,
	}
}

func (plot *plotWidget) currentRangeLocked() axisRange {
	if plot.view != nil {
		return *plot.view
	}
	result := axisRange{xMin: math.Inf(1), xMax: math.Inf(-1), yMin: math.Inf(1), yMax: math.Inf(-1)}
	for _, series := range plot.series {
		points := series.Points
		if plot.paused {
			points = plot.frozen[series.ID]
		}
		for _, point := range points {
			result.xMin = math.Min(result.xMin, point.X)
			result.xMax = math.Max(result.xMax, point.X)
			result.yMin = math.Min(result.yMin, point.Y)
			result.yMax = math.Max(result.yMax, point.Y)
		}
	}
	result.xMin, result.xMax = normalizedRange(result.xMin, result.xMax)
	result.yMin, result.yMax = normalizedRange(result.yMin, result.yMax)
	if plot.axes.X.Fixed {
		result.xMin, result.xMax = plot.axes.X.Min, plot.axes.X.Max
	}
	if plot.axes.Y.Fixed {
		result.yMin, result.yMax = plot.axes.Y.Min, plot.axes.Y.Max
	}
	return result
}

func normalizedRange(minimum float64, maximum float64) (float64, float64) {
	if math.IsInf(minimum, 0) || math.IsInf(maximum, 0) {
		return 0, 1
	}
	if minimum == maximum {
		padding := math.Abs(minimum) * 0.05
		if padding == 0 {
			padding = 0.5
		}
		return minimum - padding, maximum + padding
	}
	padding := (maximum - minimum) * 0.05
	return minimum - padding, maximum + padding
}

var _ Chart = (*plotWidget)(nil)
