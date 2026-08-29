# gnss-device-sdk

Go-библиотека с единообразными потоковыми RX/TX-интерфейсами для
радиоустройств, используемых сервисами GNSS.

## Структура

```text
gnss-device-sdk/
├── contracts.go
├── gnss-[device]/
│   ├── rx/
│   └── tx/
└── plot/                 # отдельный Go-модуль для графиков Fyne
```

Каждый адаптер должен реализовывать корневые интерфейсы `device.Receiver` и
`device.Transmitter`, чтобы потребителей не приходилось изменять при выборе
другого оборудования.

Инструкции по установке, подключению, параметрам и примеры использования
находятся только в папке конкретного устройства.

## Доступные адаптеры

- [HackRF](gnss-hackrf/README.md)
- [ADALM-Pluto](gnss-pluto/README.md)
- [UHD / USRP](gnss-uhd/README.md)

## Графики Fyne

Интерактивные GPU-графики с CPU fallback находятся в отдельном модуле
[`plot`](plot/README.md). Благодаря границе Go-модуля Fyne не становится
зависимостью приложений, использующих только SDR-адаптеры.

```bash
go get github.com/GNSS-BANK/gnss-device-sdk/plot
```

## Подключение SDK

```bash
go get github.com/GNSS-BANK/gnss-device-sdk
```

## Проверка

```bash
go test ./...
go vet ./...
go build ./...
```

Аппаратные проверки описываются в README соответствующего адаптера.
