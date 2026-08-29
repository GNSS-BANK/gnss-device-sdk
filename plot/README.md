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

Требуется Go 1.23 или новее. Модуль использует Fyne 2.8.1 только внутри
реализации: клиентскому приложению импортировать Fyne не требуется.

## Минимальный пример

```go
package main

import (
	"image/color"
	"log"

	"github.com/GNSS-BANK/gnss-device-sdk/plot"
)

func main() {
	chart, err := plot.New(
		plot.WithMaxPoints(4096),
		plot.WithMaxSeries(4),
		plot.WithBackend(plot.BackendAuto),
		plot.WithTheme(plot.ThemeDark),
		plot.WithFont(plot.FontGOSTTypeA),
		plot.WithFontSize(18),
		plot.WithMinSize(plot.NewSize(800, 450)),
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

	window, err := plot.NewWindow(chart, plot.WindowConfig{
		Title:          "Спектр",
		Size:           plot.NewSize(1100, 650),
		CenterOnScreen: true,
		Controls: plot.ControlsConfig{
			ShowPause:     true,
			ShowZoom:      true,
			ShowClear:     true,
			ShowResetZoom: true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

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
```

При `Fixed: false` диапазон оси рассчитывается по данным автоматически.
`HideGrid: true` скрывает сетку. Допустимо от 2 до 10 делений; значение 0
включает 5 делений по умолчанию. Подпись оси Y отображается вертикально слева
от значений делений. Начальная тема задаётся в конфигурации графика:

```go
chart, err := plot.New(
	plot.WithTheme(plot.ThemeDark), // либо plot.ThemeLight
)
```

`SetTheme` остаётся доступным для программного переключения темы во время
работы, но стандартная панель не содержит кнопок темы.

## Шрифты

Шрифт задаётся при создании графика и применяется ко всем подписям осей,
делениям, легенде, tooltip и стандартной панели управления:

```go
chart, err := plot.New(
	plot.WithFont(plot.FontGOSTTypeA),
	plot.WithFontSize(18),
)
```

- `plot.FontDefault` — прежний стандартный шрифт Fyne; используется по
  умолчанию;
- `plot.FontGOSTTypeA` — встроенный OpenGost Type A по ГОСТ 2.304-81.

Пользователю не требуется устанавливать шрифт в операционную систему или
передавать путь к TTF. Файл `assets/fonts/OpenGostTypeA-Regular.ttf` встроен в
модуль и распространяется по SIL Open Font License 1.1; полный текст лицензии
находится в `assets/fonts/OFL-1.1.txt`. Использован неизменённый файл из
[официального пакета openSUSE](https://software.opensuse.org/package/opengost-fonts).

`WithFontSize` принимает базовый размер от 6 до 72 логических пикселей. По
умолчанию используется `14`. Подписи осей и controls используют базовый размер,
а деления, легенда и tooltip сохраняют прежние визуальные пропорции. Вместе с
текстом автоматически масштабируются поля графика, легенда, tooltip и область
hit-testing, поэтому крупный шрифт не смещает zoom, drag или hover относительно
отрисованных данных.

## Стандартная панель управления

`NewWindow` самостоятельно создаёт Fyne-приложение, окно, график и стандартную
панель. Выбор renderer (`Auto`, `GPU`, `CPU`) присутствует всегда. Все кнопки
добавляются только по явным флагам:

```go
window, err := plot.NewWindow(chart, plot.WindowConfig{
	Title: "Realtime plot",
	Size:  plot.NewSize(1100, 650),
	Controls: plot.ControlsConfig{
		ShowPause:     true, // одна кнопка «Пауза»/«Продолжить»
		ShowZoom:      true, // «Приблизить» и «Отдалить»
		ShowClear:     true,
		ShowResetZoom: true,
		ZoomFactor:    0.8, // 0 использует значение по умолчанию
	},
})
if err != nil {
	return err
}

window.ShowAndRun()
```

Пустой `ControlsConfig{}` показывает только выбор renderer. В пользовательском
коде нет `fyne.App`, `fyne.Window`, `fyne.CanvasObject` или контейнеров Fyne.

## Масштабирование и значения точек

Колесо мыши масштабирует график относительно указателя. График можно
перетаскивать за любую точку его виджета: движение вправо/влево сдвигает X,
вверх/вниз — Y. После ручного перемещения `ResetZoom` возвращает автоматические
или зафиксированные в `AxesConfig` границы.

Zoom доступен через кнопки стандартной панели и программно:

```go
_ = chart.Zoom(0.8) // приблизить
_ = chart.Zoom(1.25) // отдалить
chart.ResetZoom()
```

При наведении не дальше 14 пикселей от точки появляется подсказка с именем
серии и значениями X/Y. Формат значений совпадает с `AxisConfig.Formatter`.

## Получение изображения

`Window.Capture` возвращает стандартный `image.Image` с текущим состоянием
окна: панелью, графиком, осями, легендой и tooltip. Fyne canvas при этом остаётся
внутренней деталью реализации:

```go
image, err := window.Capture()
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

type Window interface {
	ShowAndRun()
	Close()
	SetOnClosed(func())
	Capture() (image.Image, error)
}
```

Все экспортируемые типы модуля принадлежат `plot` либо стандартной библиотеке
Go. Конкретный Fyne-виджет не экспортируется, поэтому пользовательскому коду не
нужны импорты `fyne.io/fyne/v2` даже для создания окна, панели управления и
получения изображения.

## Проверка модуля

Автоматические проверки без дисплея выполняются с software-драйвером Fyne:

```bash
go test -tags ci ./...
go vet -tags ci ./...
go build -tags ci ./...
```

Обычную desktop-сборку проверяйте командой `go build ./...` в окружении со
стандартными системными зависимостями Fyne: рабочим C-компилятором, CGO и
OpenGL/OpenGL ES. Эти зависимости остаются внутренней деталью сборки модуля и
не требуют импортов Fyne в клиентском коде.
