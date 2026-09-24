# OPU 3D / STM32F407

Библиотека реализует версию 1 бинарного протокола из документа
`OPU_STM32_Protocol_RU_v2_2`: USB CDC (виртуальный COM/tty), 115200 8N1, кадры
`A5 5A`, little-endian, CRC-16/CCITT-FALSE. Поддержаны все команды каталога:
PING, GET_INFO, SET/GET_AXIS_CONFIG, ENABLE/DISABLE_AXIS, три MOVE, STOP,
EMERGENCY_STOP, CLEAR_EMERGENCY_STOP, GET_STATUS, оба ZERO,
GET_ELEVATION_SAFETY, SET_BRAKE_TIMING, TEST_BRAKE и REFERENCE_ELEVATION.

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
Версия кадра по-прежнему равна 1; номер прошивки 2.2 и возможности доступны
через `Info`. Перед первым движением оси угла места прочитайте
`ElevationSafety`, выключите её драйвер и задайте известный физический угол
через `ReferenceElevation`. Обычный `ZeroCommandPosition` разрешён для этой
оси только до первой привязки и только как привязка к 0°.
Для тестов и приложения с собственным открытием порта доступен
`stm32.New(port, timeout)`: `port` реализует `Read`, `Write`, `Close` и
`SetReadTimeout(time.Duration)`, например `serial.Port` из `go.bug.st/serial`.
`New` принимает порт во владение; после него вызовите `Ping` и `Info` перед
управлением движением.

Оси: `opu.Azimuth` (0), `opu.Elevation` (1), `opu.Polarization` (2).
`opu.AllAxes` (0xFF) разрешён только для `Stop`. Углы передаются целыми
тысячными долями градуса (`12500` = 12,5°). `MoveSteps` принимает
относительные импульсы STEP. Ответ MOVE означает только принятие команды:
для оси угла места завершение требует одновременно `!Status.Moving` и
`ElevationSafety.State == ElevationLocked`. Перед следующим движением также
проверьте отсутствие fault и ESTOP, наличие привязки и `CooldownMs == 0`.
Во время движения или теста отправляйте корректный PING/STATUS примерно каждые
0,5 с: отсутствие связи более 3 с вызывает аварийную остановку. Соответствие
физическому углу проверяйте отдельно.

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
настроен. `ZeroEncoder`, `ZeroCommandPosition` и `ReferenceElevation` изменяют
разные части координатного состояния; ни одна команда не ищет механический
ноль. `ElevationSafety` сообщает программное состояние тормоза, пределы,
cooldown и защёлкнутую причину аварии, но не подтверждает ток, температуру или
физическое положение штока.
