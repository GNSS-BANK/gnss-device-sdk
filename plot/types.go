// Package plot предоставляет самостоятельные интерактивные GPU-графики с CPU
// fallback. Оконная реализация скрыта за собственным публичным API пакета.
package plot

import (
	"fmt"
	"image/color"
	"math"
)

const (
	maxGPUPointSearchSteps = 14

	// MaxGPUPoints — максимальное число точек одной серии, передаваемое одному
	// GPU-шейдеру. Ограничение соответствует фиксированному 14-шаговому поиску
	// внутри GLSL и ограничивает стоимость одного кадра.
	MaxGPUPoints = 1 << maxGPUPointSearchSteps
	maxAxisTicks = 10
)

// Size задаёт логический размер графика или окна без зависимости публичного
// API от Fyne.
type Size struct {
	Width  float32
	Height float32
}

// NewSize создаёт размер в логических пикселях.
func NewSize(width float32, height float32) Size {
	return Size{Width: width, Height: height}
}

// Point — одна точка графика.
type Point struct {
	X float64
	Y float64
}

// DrawMode определяет способ отображения серии.
type DrawMode uint8

const (
	// DrawLine соединяет соседние точки сплошной линией.
	DrawLine DrawMode = iota
	// DrawPoints показывает только отдельные точки.
	DrawPoints
	// DrawPopsicle показывает точки и вертикальные ножки до нулевой линии.
	DrawPopsicle
)

// Series описывает одну независимо настраиваемую серию.
type Series struct {
	ID          string
	Name        string
	Points      []Point
	Mode        DrawMode
	Color       color.Color
	LineWidth   float32
	PointRadius float32
}

// AxisConfig настраивает одну ось. Если Fixed=false, границы вычисляются по
// данным. При Fixed=true должны быть заданы Min < Max.
type AxisConfig struct {
	Label     string
	Fixed     bool
	Min       float64
	Max       float64
	Ticks     int
	Formatter func(float64) string
}

// AxesConfig настраивает обе оси и сетку.
type AxesConfig struct {
	X        AxisConfig
	Y        AxisConfig
	HideGrid bool
}

// ThemeVariant задаёт автономную цветовую тему графика.
type ThemeVariant uint8

const (
	// ThemeDark — тёмная тема.
	ThemeDark ThemeVariant = iota
	// ThemeLight — светлая тема.
	ThemeLight
)

// FontFamily задаёт шрифт всего самостоятельного окна графика: подписей осей,
// делений, легенды, tooltip и стандартной панели управления.
type FontFamily uint8

const (
	// FontDefault сохраняет стандартный шрифт Fyne, использовавшийся ранее.
	FontDefault FontFamily = iota
	// FontGOSTTypeA использует встроенный OpenGost Type A по ГОСТ 2.304-81.
	FontGOSTTypeA
)

// RenderBackend выбирает способ отрисовки серий.
type RenderBackend uint8

const (
	// BackendAuto использует GPU в обычном Fyne-драйвере и CPU в software/test driver.
	BackendAuto RenderBackend = iota
	// BackendGPU принудительно использует canvas.Shader (OpenGL/OpenGL ES).
	BackendGPU
	// BackendCPU принудительно использует программный raster renderer.
	BackendCPU
)

func validatePoint(point Point) error {
	if math.IsNaN(point.X) || math.IsInf(point.X, 0) || math.IsNaN(point.Y) || math.IsInf(point.Y, 0) {
		return fmt.Errorf("plot point coordinates must be finite: x=%v y=%v", point.X, point.Y)
	}
	return nil
}

func validateAxes(axes AxesConfig) error {
	if err := validateAxis("X", axes.X); err != nil {
		return err
	}
	return validateAxis("Y", axes.Y)
}

func validateAxis(name string, axis AxisConfig) error {
	if axis.Ticks < 0 || axis.Ticks == 1 || axis.Ticks > maxAxisTicks {
		return fmt.Errorf("%s axis ticks must be 0 or between 2 and %d", name, maxAxisTicks)
	}
	if !axis.Fixed {
		return nil
	}
	if math.IsNaN(axis.Min) || math.IsInf(axis.Min, 0) || math.IsNaN(axis.Max) || math.IsInf(axis.Max, 0) || axis.Min >= axis.Max {
		return fmt.Errorf("fixed %s axis requires finite Min < Max", name)
	}
	return nil
}
