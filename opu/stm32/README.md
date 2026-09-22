# OPU 3D / STM32F407

Библиотека реализует версию 1 бинарного протокола из документа
`OPU_STM32_Protocol_RU`: USB CDC (виртуальный COM/tty), 115200 8N1, кадры
`A5 5A`, little-endian, CRC-16/CCITT-FALSE. Поддержаны все команды каталога:
PING, GET_INFO, SET/GET_AXIS_CONFIG, ENABLE/DISABLE_AXIS, три MOVE, STOP,
EMERGENCY_STOP, CLEAR_EMERGENCY_STOP, GET_STATUS и оба ZERO.

```go
ctx := context.Background()
unit, err := stm32.Open(ctx, "COM15") // Linux: /dev/ttyACM0
if err != nil { return err }
defer unit.Close()

status, err := unit.Status(ctx, opu.Azimuth)
if err != nil { return err }
_ = status
```

`Open` проверяет PING и GET_INFO, включая версию протокола и число осей.
Для тестов и приложения с собственным открытием порта доступен
`stm32.New(port, timeout)`: `port` реализует `Read`, `Write`, `Close` и
`SetReadTimeout(time.Duration)`, например `serial.Port` из `go.bug.st/serial`.
`New` принимает порт во владение; после него вызовите `Ping` и `Info` перед
управлением движением.

Оси: `opu.Azimuth` (0), `opu.Elevation` (1), `opu.Polarization` (2).
`opu.AllAxes` (0xFF) разрешён только для `Stop`. Углы передаются целыми
тысячными долями градуса (`12500` = 12,5°). `MoveSteps` принимает
относительные импульсы STEP. Ответ MOVE означает только принятие команды:
для окончания движения опрашивайте `Status` до `!Moving` и
`RemainingSteps == 0`; соответствие физическому углу проверяйте отдельно.

Каждый вызов сериализуется общей блокировкой, последовательность запроса
проверяется в ответе, повреждённые кадры отбрасываются. Время ожидания ответа
по умолчанию 2 с. Клиент не повторяет команды автоматически: после тайм-аута
или ошибки записи выполнение уже отправленной команды неизвестно. Буфер
частичного ответа при тайм-ауте очищается; повторное открытие порта само по
себе не сбрасывает парсер прошивки. Конфигурация осей хранится в RAM STM32 и
после его перезапуска должна быть установлена снова.

Пример чтения с учётом ошибок прошивки:

```go
config, err := unit.AxisConfig(ctx, opu.Elevation)
var result *stm32.ResultError
if errors.As(err, &result) && result.Code == stm32.ResultBusy {
    // Ось сейчас движется.
}
_ = config
```

`Status.EncoderAngleMdeg == math.MinInt32` означает, что масштаб энкодера не
настроен. `ZeroEncoder` и `ZeroCommandPosition` обнуляют разные координаты;
ни одна команда не ищет механический ноль. Протокол не обеспечивает
автоматическую остановку при отключении USB.
