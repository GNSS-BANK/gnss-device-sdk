# ADALM-Pluto

Адаптер предоставляет единообразные интерфейсы SDK для потокового приёма и
передачи через ADALM-Pluto. Для связи используются официальные утилиты
`libiio`: `iio_attr`, `iio_readdev` и `iio_writedev`. CGO и сторонние
Go-модули не требуются.

## Подготовка

1. Установите `libiio` вместе с CLI-утилитами и добавьте их в `PATH`.
2. Подключите Pluto по USB или сети.
3. Найдите URI доступного IIO-контекста:

```bash
iio_info -s
```

Примеры URI:

- `ip:192.168.2.1` — стандартный USB Ethernet-адрес Pluto;
- `ip:pluto.local` — подключение по имени;
- `usb:3.8.5` — URI конкретного USB-контекста из вывода `iio_info -s`;
- `local:` — выполнение непосредственно на Linux-системе Pluto.

Проверьте выбранное устройство:

```bash
iio_info -u ip:192.168.2.1
```

В этом адаптере поле `StreamConfig.DeviceID` обязательно и содержит именно
этот URI. Благодаря этому при нескольких Pluto выбор устройства остаётся
явным.

## Формат данных

`SampleFormat()` возвращает `device.SampleFormatComplexInt16LE`. Каждый
комплексный отсчёт занимает 4 байта:

```text
I0 int16 LE, Q0 int16 LE, I1 int16 LE, Q1 int16 LE, ...
```

При приёме Pluto выдаёт знаковые 12-битные значения в 16-битных контейнерах
(`le:S12/16>>0`). При передаче принимаются знаковые 16-битные значения
(`le:S16/16>>0`). Файл другого формата — например WAV, NumPy `complex64` или
IQ `int8` от HackRF — перед передачей нужно преобразовать.

`SampleCount` задаётся в комплексных отсчётах, а не в байтах. Значение `0`
означает поток до EOF входного файла, остановки устройства или отмены
контекста.

## Приём в файл

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	plutorx "github.com/GNSS-BANK/gnss-device-sdk/gnss-pluto/rx"
)

func main() {
	dst, err := os.Create("capture.iq")
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	receiver := plutorx.New()
	err = receiver.Read(context.Background(), dst, device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "ip:192.168.2.1",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      3_000_000,
			BandwidthHz:       2_500_000,
			SampleCount:       3_000_000,
		},
		Gains: []device.Gain{{Stage: "HARDWARE", ValueDB: 20}},
	})
	if err != nil {
		panic(err)
	}
}
```

Если `Gains` не задан, текущее значение и режим усиления устройства не
изменяются. Если задан `HARDWARE`, адаптер сначала включает ручной RX gain.

## Передача IQ-файла

Передавать можно не произвольный файл, а сырой IQ-поток описанного выше
формата. При `SampleCount: 0` весь файл будет однократно прочитан до EOF:

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	plutotx "github.com/GNSS-BANK/gnss-device-sdk/gnss-pluto/tx"
)

func main() {
	src, err := os.Open("signal.iq")
	if err != nil {
		panic(err)
	}
	defer src.Close()

	transmitter := plutotx.New()
	err = transmitter.Write(context.Background(), src, device.TXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "ip:192.168.2.1",
			CenterFrequencyHz: 1_227_600_000,
			SampleRateHz:      3_000_000,
			BandwidthHz:       2_500_000,
			SampleCount:       0,
		},
		Gains: []device.Gain{{Stage: "HARDWARE", ValueDB: -30}},
	})
	if err != nil {
		panic(err)
	}
}
```

Передачу выполняйте только на разрешённой частоте и мощности. Для GNSS- и
других защищённых диапазонов используйте закрытый стенд с экранированием,
кабельным подключением и аттенюатором, чтобы сигнал не вышел в эфир.

## Параметры

Для штатного AD9363 без расширения диапазона адаптер проверяет:

| Параметр              |                      RX |                       TX |
| --------------------- | ----------------------: | -----------------------: |
| Центральная частота   |         325 МГц–3,8 ГГц |          325 МГц–3,8 ГГц |
| Частота дискретизации | 2 083 333–61 440 000 Гц |  2 083 333–61 440 000 Гц |
| Полоса                |          200 кГц–56 МГц |           200 кГц–40 МГц |
| `HARDWARE` gain       |      −3…71 дБ, шаг 1 дБ | −89,75…0 дБ, шаг 0,25 дБ |

При `BandwidthHz: 0` текущая полоса Pluto не изменяется. Поля
`RFAmplifierEnabled`, `AntennaPowerEnabled` и `HardwareTrigger` относятся к
другим устройствам; значение `true` для Pluto возвращает ошибку.

По умолчанию IIO-буфер содержит 32 768 комплексных отсчётов. Его можно
изменить отдельно для RX или TX:

```go
receiver := plutorx.New(plutorx.WithBufferSize(65_536))
transmitter := plutotx.New(plutotx.WithBufferSize(65_536))
```

Пути к утилитам и вывод диагностики также настраиваются constructor options:

```go
receiver := plutorx.New(
	plutorx.WithIIOAttrBinary("/opt/libiio/bin/iio_attr"),
	plutorx.WithIIOReaddevBinary("/opt/libiio/bin/iio_readdev"),
	plutorx.WithStderr(os.Stderr),
)

transmitter := plutotx.New(
	plutotx.WithIIOAttrBinary("/opt/libiio/bin/iio_attr"),
	plutotx.WithIIOWritedevBinary("/opt/libiio/bin/iio_writedev"),
	plutotx.WithStderr(os.Stderr),
)
```

Одна операция состоит из нескольких последовательных CLI-вызовов: сначала
настраиваются sample rate, полоса, LO и gain, затем запускается поток. Не
запускайте конкурирующие RX/TX-операции, меняющие настройки одного Pluto, без
внешней синхронизации. Если поздний шаг завершился ошибкой, уже применённые к
устройству настройки не откатываются автоматически.

## Остановка

Для управляемой остановки передайте отменяемый context:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

err := receiver.Read(ctx, dst, config)
```

Отмена завершает активную утилиту `libiio` и возвращает `context.Canceled`.

## Проверка

```bash
go test ./gnss-pluto/...
```

Unit-тесты не требуют устройства. Аппаратную RX-проверку выполняйте с
ограниченным `SampleCount`; TX-проверку — только на безопасном закрытом
радиостенде.

## Справочные материалы

- [CLI-утилиты libiio](https://analogdevicesinc.github.io/documentation/software/libiio/cli.html)
- [Подключение Pluto через libiio](https://analogdevicesinc.github.io/documentation/solutions/platforms/pluto/setup/libiio.html)
