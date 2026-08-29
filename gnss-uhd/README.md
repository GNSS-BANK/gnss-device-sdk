# UHD / USRP

Адаптер `gnss-uhd` реализует единообразные RX/TX-интерфейсы SDK для
устройств, поддерживаемых UHD: в том числе USRP X300/X310. Поток передаётся в
нативном формате UHD SC16 через Go-библиотеку `hz.tools/sdr/uhd`.

## Требования

Адаптер работает только в Linux-сборке с включённым CGO. На хосте нужны:

- UHD и development-файлы (`libuhd-dev` в Debian/Ubuntu);
- C/C++ toolchain;
- `pkg-config` и доступный через него `uhd.pc`;
- сетевой или USB-доступ к выбранному USRP.

На других платформах пакеты собираются, но вызовы `Read` и `Write` возвращают
ошибку о необходимости Linux/CGO/UHD.

## Выбор устройства

Найдите устройства и проверьте соединение штатными командами UHD:

```bash
uhd_find_devices
uhd_usrp_probe --args "type=x300,addr=192.168.10.2"
```

Поле `StreamConfig.DeviceID` содержит строку device args UHD целиком. Примеры:

- `type=x300,addr=192.168.10.2` — X300/X310 по IP;
- `serial=ABC123` — устройство с конкретным серийным номером;
- `type=x300,addr=192.168.10.2,second_addr=192.168.20.2` — двухпортовое подключение.

Поле обязательно: адаптер не выбирает первое найденное устройство неявно.
Формат и доступные ключи зависят от модели и описаны в документации UHD.

## Формат данных

`SampleFormat()` возвращает `device.SampleFormatComplexInt16LE`. Один
комплексный отсчёт занимает 4 байта:

```text
I0 int16 LE, Q0 int16 LE, I1 int16 LE, Q1 int16 LE, ...
```

RX записывает именно такой сырой поток, а TX ожидает его на входе. WAV,
NumPy `complex64`, CI8 от HackRF и другие форматы перед передачей нужно
преобразовать в SC16 little-endian. `SampleCount` измеряется в комплексных
отсчётах, а не в байтах; `0` означает поток до EOF или отмены контекста.

## Приём в файл

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	uhdrx "github.com/GNSS-BANK/gnss-device-sdk/gnss-uhd/rx"
)

func main() {
	dst, err := os.Create("capture.sc16")
	if err != nil {
		panic(err)
	}
	defer dst.Close()

	receiver := uhdrx.New(
		uhdrx.WithChannel(0),
		uhdrx.WithAutomaticGain(false),
	)
	err = receiver.Read(context.Background(), dst, device.RXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "type=x300,addr=192.168.10.2",
			CenterFrequencyHz: 1_575_420_000,
			SampleRateHz:      10_000_000,
			SampleCount:       10_000_000,
		},
		Gains: []device.Gain{{Stage: "PGA", ValueDB: 20}},
	})
	if err != nil {
		panic(err)
	}
}
```

Названия и диапазоны gain stages адаптер получает от конкретного устройства
во время выполнения. Можно передать полное имя из UHD либо однозначный суффикс
вроде `PGA`. Если имя не найдено или неоднозначно, ошибка перечислит доступные
каскады.

RX использует ограниченный переиспользуемый буфер. По умолчанию это 256 МиБ,
блоки по 262 144 комплексных отсчёта. Если получатель не успевает сохранять
поток и буфер заполняется, операция завершается ошибкой вместо молчаливой
потери части записи.

## Передача файла

Если файл уже содержит сырой SC16 little-endian, его можно передать напрямую:

```go
package main

import (
	"context"
	"os"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	uhdtx "github.com/GNSS-BANK/gnss-device-sdk/gnss-uhd/tx"
)

func main() {
	src, err := os.Open("signal.sc16")
	if err != nil {
		panic(err)
	}
	defer src.Close()

	transmitter := uhdtx.New(uhdtx.WithChannel(0))
	err = transmitter.Write(context.Background(), src, device.TXConfig{
		StreamConfig: device.StreamConfig{
			DeviceID:          "type=x300,addr=192.168.10.2",
			CenterFrequencyHz: 1_227_600_000,
			SampleRateHz:      10_000_000,
			SampleCount:       0,
		},
		Gains: []device.Gain{{Stage: "PGA", ValueDB: 10}},
	})
	if err != nil {
		panic(err)
	}
}
```

Передача создаёт радиочастотное излучение. Для GNSS- и других защищённых
диапазонов используйте только разрешённый закрытый стенд: кабельное
подключение, аттенюатор, экранирование и подходящую нагрузку.

## Параметры адаптера

| Параметр | Назначение |
| --- | --- |
| `DeviceID` | Обязательная строка UHD device args |
| `CenterFrequencyHz` | Центральная частота; диапазон проверяет UHD и устройство |
| `SampleRateHz` | Частота дискретизации; диапазон проверяет UHD и устройство |
| `BandwidthHz` | Пока не поддерживается и должен быть равен `0` |
| `SampleCount` | Число комплексных отсчётов; `0` — до остановки/EOF |
| `Gains` | Именованные RX- или TX-каскады и значения в дБ |

Поля `RFAmplifierEnabled`, `AntennaPowerEnabled` и `HardwareTrigger` этим
адаптером не поддерживаются. Значение `true` возвращает ошибку.

Дополнительные настройки конструктора:

- `rx.WithChannel` / `tx.WithChannel` — номер канала, начиная с нуля;
- `rx.WithBufferLength` / `tx.WithBufferLength` — внутренний параметр буфера UHD;
- `rx.WithChunkSize` / `tx.WithChunkSize` — размер блока в комплексных отсчётах;
- `rx.WithRXBufferSize` — общий объём ограниченного RX-буфера в байтах;
- `rx.WithAutomaticGain` — явное включение или отключение AGC;
- `WithSettleDelay` — пауза после настройки частоты и усиления;
- `rx.WithRestartDelay` — пауза перед повтором RX после восстанавливаемой ошибки до первого отсчёта.

Если `WithAutomaticGain` не указан, адаптер не изменяет текущее состояние AGC.
Одновременно включать AGC и передавать ручные `Gains` нельзя.

## Проверка

Unit-тесты не требуют подключённого устройства:

```bash
go test ./gnss-uhd/...
```

Для проверки Linux/CGO-сборки и оборудования:

```bash
pkg-config --exists uhd
CGO_ENABLED=1 go test ./gnss-uhd/...
```

Аппаратную RX-проверку начинайте с малого `SampleCount`. Для X300/X310 на
высокой частоте дискретизации отдельно проверьте непрерывную минутную запись,
скорость диска и отсутствие overflow. TX проверяйте только на безопасном
закрытом радиостенде.
