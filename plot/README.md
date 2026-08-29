# plot

Интерактивный Fyne-график для GNSS-приложений. Модуль умеет отображать
несколько статических и потоковых серий, масштабироваться, ставить отображение
на паузу, показывать легенду и значение ближайшей точки. Серии рисуются на GPU
через `canvas.Shader`; для тестовых, программных и несовместимых окружений есть
CPU fallback.

`plot` является отдельным Go-модулем, поэтому приложения, которым нужны только
драйверы SDR, не получают Fyne и графические зависимости.

## Подключение

```bash
go get github.com/GNSS-BANK/gnss-device-sdk/plot
```

Требуется Go 1.23 или новее. Модуль использует Fyne 2.8.1.

## Минимальный пример

```go
package main

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2/app"
	"github.com/GNSS-BANK/gnss-device-sdk/plot"
)

func main() {
	application := app.New()
	window := application.NewWindow("Спектр")

	chart, err := plot.New(
		plot.WithMaxPoints(4096),
		plot.WithMaxSeries(4),
		plot.WithBackend(plot.BackendAuto),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = chart.SetSeries([]plot.Series{
		{
			ID:    "spectrum",
			Name:  "Спектр",
			Mode:  plot.DrawLine,
			Color: color.NRGBA{R: 68, G: 138, B: 255, A: 255},
			Points: []plot.Point{
				{X: 1_575_400_000, Y: -73.2},
				{X: 1_575_420_000, Y: -61.8},
				{X: 1_575_440_000, Y: -70.4},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	window.SetContent(chart.Object())
	window.ShowAndRun()
}
```

Если `Color` не задан, библиотека выберет цвет из встроенной палитры. Если
`Name` пуст, в легенде используется `ID`.

## Потоковые данные

Сначала зарегистрируйте серию, затем добавляйте новые точки через `Append`:

```go
if err := chart.AddSeries(plot.Series{
	ID:   "power",
	Name: "Мощность",
	Mode: plot.DrawLine,
}); err != nil {
	return err
}

go func() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for now := range ticker.C {
		point := plot.Point{
			X: float64(now.UnixMilli()),
			Y: readPower(),
		}
		if err := chart.Append("power", point); err != nil {
			log.Printf("append point: %v", err)
		}
	}
}()
```

`Append` потокобезопасен и сам планирует обновление в UI-горутины Fyne.
Значения X внутри потоковой серии должны идти по неубыванию. Когда достигнут
лимит `WithMaxPoints`, старые точки автоматически удаляются.

Пауза замораживает только отображение. Новые точки продолжают накапливаться и
появятся после `Resume`:

```go
chart.Pause()
chart.Resume()
chart.Clear()                 // очистить все серии
chart.ClearSeries("power")    // очистить одну серию
```

## Режимы отображения

У каждой серии независимо задаётся `Mode`:

- `plot.DrawLine` — сплошная линия;
- `plot.DrawPoints` — отдельные точки;
- `plot.DrawPopsicle` — точки с вертикальными ножками до нулевой линии.

Можно одновременно показывать серии с разными режимами. Толщина линии и радиус
точки настраиваются полями `LineWidth` и `PointRadius`.

## Оси, легенда и тема

```go
err := chart.SetAxes(plot.AxesConfig{
	X: plot.AxisConfig{
		Label: "Частота",
		Fixed: true,
		Min:   1_575_000_000,
		Max:   1_576_000_000,
		Ticks: 6,
		Formatter: func(value float64) string {
			return fmt.Sprintf("%.2f МГц", value/1e6)
		},
	},
	Y: plot.AxisConfig{
		Label: "Мощность, dBFS",
		Fixed: true,
		Min:   -120,
		Max:   0,
		Ticks: 7,
	},
})
if err != nil {
	return err
}

chart.SetLegendVisible(true)
_ = chart.SetTheme(plot.ThemeDark) // либо plot.ThemeLight
```

При `Fixed: false` диапазон оси рассчитывается по данным автоматически.
`HideGrid: true` скрывает сетку. Допустимо от 2 до 10 делений; значение 0
включает 5 делений по умолчанию.

## Масштабирование и значения точек

Колесо мыши масштабирует график относительно указателя. То же самое можно
сделать программно:

```go
_ = chart.Zoom(0.8) // приблизить
_ = chart.Zoom(1.25) // отдалить
chart.ResetZoom()
```

При наведении не дальше 14 пикселей от точки появляется подсказка с именем
серии и значениями X/Y. Формат значений совпадает с `AxisConfig.Formatter`.

## Получение изображения

`Capture` возвращает `image.Image` с текущим состоянием Fyne canvas. Если
нужно изображение только графика, установите его единственным content окна.
Вызывать `Capture` следует из UI-горутины, например из обработчика кнопки:

```go
image, err := chart.Capture(window.Canvas())
if err != nil {
	return err
}

file, err := os.Create("spectrum.png")
if err != nil {
	return err
}
defer file.Close()

if err := png.Encode(file, image); err != nil {
	return err
}
```

В GPU-режиме это снимок результата OpenGL/OpenGL ES; в CPU-режиме — снимок
программного raster renderer вместе с осями, легендой и подсказкой.

## GPU и CPU fallback

Режим выбирается при создании или во время работы:

```go
chart, _ := plot.New(plot.WithBackend(plot.BackendAuto))

_ = chart.SetBackend(plot.BackendGPU)
_ = chart.SetBackend(plot.BackendCPU)
```

- `BackendAuto` — GPU в обычном оконном драйвере Fyne, CPU в test/software
  driver;
- `BackendGPU` — принудительная отрисовка серий через GLSL `canvas.Shader`;
- `BackendCPU` — принудительная отрисовка серий стандартными средствами Go в
  `canvas.Raster`.

У Fyne нет публичного callback для ошибки компиляции конкретного shader уже
запущенным драйвером. Поэтому на машине, где обычный драйвер стартовал, но GLSL
не поддерживается или компиляция shader завершилась ошибкой, переключите график
на `BackendCPU`. Данные, zoom, пауза и настройки при переключении сохраняются.

## Ограничения

- не более 4096 точек на одну серию;
- не более 16 одновременно зарегистрированных серий;
- по умолчанию: 2048 точек и 8 серий;
- `Append` принимает только неубывающий X;
- GPU-ускорение относится к отрисовке серий; оси, подписи, легенда и tooltip —
  стандартные canvas-примитивы Fyne и выводятся активным painter Fyne.

## Интерфейс

Публичный интерфейс `Chart` позволяет подменить реализацию в приложении или в
тестах:

```go
type Chart interface {
	Object() fyne.CanvasObject
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
	Capture(fyne.Canvas) (image.Image, error)
}
```

## Проверка модуля

```bash
go test ./...
go vet ./...
go build ./...
```
