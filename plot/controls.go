package plot

import (
	"errors"
	"math"
	"reflect"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ControlsConfig выбирает кнопки стандартной панели управления. Выбор
// renderer присутствует всегда и поэтому отдельного флага не имеет.
type ControlsConfig struct {
	ShowPause     bool
	ShowZoom      bool
	ShowClear     bool
	ShowResetZoom bool

	// ZoomFactor задаёт коэффициент кнопки приближения: значение от 0 до 1.
	// Кнопка отдаления использует обратный коэффициент. Ноль означает 0.8.
	ZoomFactor float64
}

// NewControls создаёт стандартную панель графика. Renderer selector выводится
// всегда, остальные кнопки включаются через ControlsConfig.
func NewControls(chart Chart, config ControlsConfig) (*fyne.Container, error) {
	if chartIsNil(chart) {
		return nil, errors.New("plot chart is nil")
	}
	zoomFactor := config.ZoomFactor
	if zoomFactor == 0 {
		zoomFactor = 0.8
	}
	if math.IsNaN(zoomFactor) || math.IsInf(zoomFactor, 0) || zoomFactor <= 0 || zoomFactor >= 1 {
		return nil, errors.New("plot controls zoom factor must be finite and between 0 and 1")
	}

	objects := make([]fyne.CanvasObject, 0, 8)
	if config.ShowPause {
		var pause *widget.Button
		pause = widget.NewButton(pauseButtonText(chart.Paused()), func() {
			if chart.Paused() {
				chart.Resume()
			} else {
				chart.Pause()
			}
			pause.SetText(pauseButtonText(chart.Paused()))
		})
		objects = append(objects, pause)
	}
	if config.ShowZoom {
		objects = append(objects,
			widget.NewButton("Приблизить", func() {
				_ = chart.Zoom(zoomFactor)
			}),
			widget.NewButton("Отдалить", func() {
				_ = chart.Zoom(1 / zoomFactor)
			}),
		)
	}
	if config.ShowClear {
		objects = append(objects, widget.NewButton("Очистить", chart.Clear))
	}
	if config.ShowResetZoom {
		objects = append(objects, widget.NewButton("Сбросить zoom", chart.ResetZoom))
	}

	if len(objects) > 0 {
		objects = append(objects, widget.NewSeparator())
	}
	objects = append(objects, widget.NewLabel("Renderer:"))
	backend := widget.NewSelect([]string{"Auto", "GPU", "CPU"}, func(selected string) {
		_ = chart.SetBackend(backendFromLabel(selected))
	})
	backend.SetSelected(backendLabel(chart.Backend()))
	objects = append(objects, backend)
	return container.NewHBox(objects...), nil
}

// NewView объединяет стандартную панель и график в готовый Fyne-контейнер.
// Для графика без панели используйте chart.Object().
func NewView(chart Chart, config ControlsConfig) (*fyne.Container, error) {
	controls, err := NewControls(chart, config)
	if err != nil {
		return nil, err
	}
	return container.NewBorder(controls, nil, nil, nil, chart.Object()), nil
}

func pauseButtonText(paused bool) string {
	if paused {
		return "Продолжить"
	}
	return "Пауза"
}

func backendLabel(backend RenderBackend) string {
	switch backend {
	case BackendGPU:
		return "GPU"
	case BackendCPU:
		return "CPU"
	default:
		return "Auto"
	}
}

func backendFromLabel(label string) RenderBackend {
	switch label {
	case "GPU":
		return BackendGPU
	case "CPU":
		return BackendCPU
	default:
		return BackendAuto
	}
}

func chartIsNil(chart Chart) bool {
	if chart == nil {
		return true
	}
	value := reflect.ValueOf(chart)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
