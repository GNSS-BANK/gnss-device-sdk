# gnss-device-sdk

Go-библиотека с единообразными потоковыми RX/TX-интерфейсами для
радиоустройств, используемых сервисами GNSS.

## Структура

```text
gnss-device-sdk/
├── contracts.go
└── gnss-[device]/
    ├── rx/
    └── tx/
```

Первый адаптер — `gnss-hackrf`. Адаптеры следующих устройств должны
реализовывать те же корневые интерфейсы `device.Receiver` и
`device.Transmitter`, чтобы потребителей не приходилось изменять.

## Приём данных с HackRF

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

## Передача данных через HackRF

```go
input, err := os.Open("signal.ci8")
if err != nil {
	panic(err)
}
defer input.Close()

transmitter := tx.New()
err = transmitter.Write(context.Background(), input, device.TXConfig{
	StreamConfig: device.StreamConfig{
		CenterFrequencyHz: 1_575_420_000,
		SampleRateHz:      10_000_000,
	},
	Gains: []device.Gain{{Stage: "VGA", ValueDB: 12}},
})
```

В режиме TX HackRF излучает радиочастотную энергию. Перед подключением антенны
используйте необходимое ослабление и экранирование, а также убедитесь, что
передача разрешена местными требованиями.

## Проверка

```bash
go test ./...
go vet ./...
go build ./...
```

Для аппаратной проверки дополнительно нужны подключённый HackRF и официальная
утилита `hackrf_transfer`.
