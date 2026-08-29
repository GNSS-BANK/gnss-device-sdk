package plot

import (
	"errors"
	"image"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

// WindowConfig настраивает самостоятельное окно графика. Все Fyne-объекты
// создаются внутри библиотеки и не попадают в код приложения.
type WindowConfig struct {
	Title          string
	Size           Size
	CenterOnScreen bool
	Controls       ControlsConfig
}

// Window управляет самостоятельным окном графика без Fyne-типов в публичном
// интерфейсе.
type Window interface {
	ShowAndRun()
	Close()
	SetOnClosed(func())
	Capture() (image.Image, error)
}

type plotWindow struct {
	application fyne.App
	window      fyne.Window
	plot        *plotWidget
}

// NewWindow создаёт Fyne-приложение, окно, панель управления и связывает их с
// графиком. Для одного процесса следует создавать одно самостоятельное окно.
func NewWindow(chart Chart, config WindowConfig) (Window, error) {
	if chartIsNil(chart) {
		return nil, errors.New("plot chart is nil")
	}
	plot, ok := chart.(*plotWidget)
	if !ok {
		return nil, errors.New("plot window requires a chart created by plot.New")
	}

	size := config.Size
	if size.Width == 0 && size.Height == 0 {
		size = NewSize(1000, 600)
	}
	if size.Width <= 0 || size.Height <= 0 {
		return nil, errors.New("plot window size must be positive")
	}
	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "Plot"
	}

	application := app.New()
	if err := applyApplicationFont(application, plot.font); err != nil {
		return nil, err
	}
	window := application.NewWindow(title)
	view, err := newView(plot, config.Controls)
	if err != nil {
		return nil, err
	}
	window.SetContent(view)
	window.Resize(fyne.NewSize(size.Width, size.Height))
	if config.CenterOnScreen {
		window.CenterOnScreen()
	}
	return &plotWindow{application: application, window: window, plot: plot}, nil
}

func (window *plotWindow) ShowAndRun() {
	if window == nil || window.window == nil {
		return
	}
	window.window.ShowAndRun()
}

func (window *plotWindow) Close() {
	if window == nil || window.window == nil {
		return
	}
	window.window.Close()
}

func (window *plotWindow) SetOnClosed(callback func()) {
	if window == nil || window.window == nil {
		return
	}
	window.window.SetOnClosed(callback)
}

// Capture возвращает изображение всего окна: панели, графика, осей, легенды и
// tooltip. Метод следует вызывать после показа окна.
func (window *plotWindow) Capture() (image.Image, error) {
	if window == nil || window.window == nil || window.plot == nil {
		return nil, errors.New("plot window is nil")
	}
	window.plot.Refresh()
	result := window.window.Canvas().Capture()
	if result == nil || result.Bounds().Empty() {
		return nil, errors.New("plot window returned an empty image")
	}
	return result, nil
}

var _ Window = (*plotWindow)(nil)
