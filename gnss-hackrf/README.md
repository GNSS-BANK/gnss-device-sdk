# Адаптер HackRF

`gnss-hackrf` реализует общие интерфейсы SDK через официальную утилиту
`hackrf_transfer`. Утилита должна быть установлена на хосте и доступна через
`PATH`. Другой путь можно передать с помощью `rx.WithBinary` или
`tx.WithBinary`.

## Пакеты

- `github.com/GNSS-BANK/gnss-device-sdk/gnss-hackrf/rx` — приём чередующихся знаковых 8-битных IQ-отсчётов;
- `github.com/GNSS-BANK/gnss-device-sdk/gnss-hackrf/tx` — передача чередующихся знаковых 8-битных IQ-отсчётов.

## Выбор устройства

HackRF подключается по USB и выбирается по серийному номеру. Список
подключённых устройств и их серийные номера выводит команда:

```bash
hackrf_info
```

Серийный номер передаётся в `StreamConfig.DeviceID`:

```go
StreamConfig: device.StreamConfig{
	DeviceID:          "0000000000000000123456789abcdef0",
	CenterFrequencyHz: 1_575_420_000,
	SampleRateHz:      10_000_000,
}
```

SDK преобразует `DeviceID` в аргумент `hackrf_transfer -d <serial_number>`.
Если `DeviceID` пуст, устройство выбирает сама утилита `hackrf_transfer`. При
одновременном подключении нескольких HackRF серийный номер следует указывать
явно.

## Приём данных

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-hackrf/rx"
)

func main() {
	output, err := os.Create("capture.ci8")
	if err != nil {
		panic(err)
	}
	defer output.Close()

	receiver := rx.New()
	err = receiver.Read(context.Background(), output, device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "0000000000000000123456789abcdef0",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
			SampleCount:       10_000_000,
		},
		Gains: []device.Gain{
			{Stage: "LNA", ValueDB: 16},
			{Stage: "VGA", ValueDB: 20},
		},
	})
	if err != nil {
		panic(err)
	}
}
```

`Read` записывает чередующиеся знаковые 8-битные IQ-отсчёты в любой
`io.Writer`: файл, сетевой поток или объектное хранилище.

## Передача данных

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-hackrf/tx"
)

func main() {
	input, err := os.Open("signal.ci8")
	if err != nil {
		panic(err)
	}
	defer input.Close()

	transmitter := tx.New()
	err = transmitter.Write(context.Background(), input, device.TXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "0000000000000000123456789abcdef0",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
		},
		Gains: []device.Gain{{Stage: "VGA", ValueDB: 12}},
	})
	if err != nil {
		panic(err)
	}
}
```

В режиме TX HackRF излучает радиочастотную энергию. Перед подключением антенны
используйте необходимое ослабление и экранирование, а также убедитесь, что
передача разрешена местными требованиями.

## Параметры

| Поле | Назначение |
| --- | --- |
| `DeviceID` | USB-серийный номер HackRF; пустое значение оставляет выбор утилите |
| `CenterFrequencyHz` | Центральная частота, от 1 МГц до 6 ГГц |
| `SampleRateHz` | Частота дискретизации, от 2 до 20 МГц |
| `BandwidthHz` | Полоса базового фильтра; `0` выбирает значение автоматически |
| `SampleCount` | Количество комплексных отсчётов; `0` означает непрерывный поток |
| `RFAmplifierEnabled` | Включение встроенного RF-усилителя |
| `AntennaPowerEnabled` | Подача питания на антенный порт |
| `HardwareTrigger` | Ожидание внешнего аппаратного триггера |
| `Gains` | Значения поддерживаемых каскадов усиления |

Поддерживаемые каскады усиления:

- RX: `LNA` — 0–40 дБ с шагом 8 дБ; `VGA` — 0–62 дБ с шагом 2 дБ;
- TX: `VGA` — 0–47 дБ с шагом 1 дБ.

Значения усиления по умолчанию совпадают с `hackrf_transfer`: RX LNA — 8 дБ,
RX VGA — 20 дБ, TX VGA — 0 дБ. Нулевая полоса позволяет HackRF выбрать
значение по умолчанию. Нулевое количество отсчётов продолжает поток до отмены
контекста или окончания входных данных.

Допустимые диапазоны частоты, частоты дискретизации, полосы и усиления
соответствуют
[официальной утилите HackRF](https://github.com/greatscottgadgets/hackrf/blob/main/host/hackrf-tools/src/hackrf_transfer.c).

## Изменение пути к утилите

Если `hackrf_transfer` отсутствует в `PATH`, путь задаётся при создании RX или
TX:

```go
receiver := rx.New(rx.WithBinary("/opt/hackrf/bin/hackrf_transfer"))
transmitter := tx.New(tx.WithBinary("/opt/hackrf/bin/hackrf_transfer"))
```

Диагностический stderr утилиты можно получить через `rx.WithStderr` или
`tx.WithStderr`.

## Проверка

Автоматические проверки запускаются из корня `gnss-device-sdk`:

```bash
go test ./...
go vet ./...
go build ./...
```

Для аппаратной проверки нужны подключённый HackRF и установленная утилита
`hackrf_transfer`. RX следует проверять с ограниченным `SampleCount`. TX можно
проверять только на безопасном стенде с необходимым ослаблением, экранированием
и разрешённой частотой.
