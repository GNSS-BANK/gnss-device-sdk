# OPU 3D / STM32F407

SDK реализует бинарный wire-протокол 1 по редакции 20 октября 2026 года
`OPU_STM32_Protocol_RU_2.docx`: прошивка 3.0 без тормоза, build ID
`0xDF405E70`. USB CDC, 115200 8N1, кадры `A5 5A`, little-endian,
CRC-16/CCITT-FALSE. Модель `stm32-opu-3d-v1` обозначает формат кадра.

Поддержаны PING, GET_INFO, GET_DIAGNOSTICS, SET/GET_AXIS_CONFIG,
ENABLE/DISABLE_AXIS, MOVE_STEPS, MOVE_REL_ANGLE, MOVE_ABS_ANGLE,
MOVE_ABS_UNWRAPPED, STOP, EMERGENCY_STOP, CLEAR_EMERGENCY_STOP, GET_STATUS,
оба ZERO, REFERENCE_ELEVATION, GET_TARGET и SET_LINK_TIMEOUT.
SET_BRAKE_TIMING и TEST_BRAKE сохранены для старых прошивок 2.2; прошивка 3.0
возвращает UNSUPPORTED. GET_ELEVATION_SAFETY оставлен для совместимости:
нулевые state/coil/cooldown/timing не подтверждают остановку.

```go
ctx := context.Background()
unit, err := stm32.Open(ctx, "COM15") // Linux: /dev/ttyACM0
if err != nil { return err }
defer unit.Close()
info, err := unit.Info(ctx)
if err != nil { return err }
diag, err := unit.Diagnostics(ctx)
if err != nil { return err }
_ = info // Для этой поставки: major=3, protocol=1, axes=3.
_ = diag // Сверить BuildID, USBSessions, UptimeMs, fault и ESTOP.
```

`Open` сначала проверяет GET_INFO (до трёх попыток чтения), затем PING.
Он не меняет привязку, настройки или разрешение драйверов. `New(port, timeout)`
принимает открытый порт во владение; вызывающий сам выполняет handshake.
Все запросы сериализуются mutex; начальный sequence выбирается случайно,
для каждого запроса увеличивается u16. Ответ проверяется по cmd/seq/CRC/длине.
Таймаут ответа по умолчанию 2 с, чтения COM — 50 мс. Движения не повторяются
автоматически: после таймаута выполнение неизвестно. Кеш прошивки ограничен
8 изменяющими запросами и 10 с; конфликт seq возвращает
`ResultSequenceConflict` (17), а reset/новая USB-сессия очищают кеш.

Оси: AZ=0, УМ=1, POL=2; FF допустим только для STOP. Угол — signed int32
в миллиградусах (12500 = 12,5°), STEP — signed int64.
`MoveAbsoluteAngle` (0x24) на AZ выбирает ближайший эквивалент: из нуля
270° даёт −90°, а 360° — 0°. `MoveAbsoluteUnwrapped` (0x44) сохраняет
развёрнутую цель: +270° и +360° соответственно. Клиент контролирует кабельный
маршрут. УМ имеет пределы −25…90°. ABS может менять цель на ходу; REL/STEPS
на занятой оси возвращают BUSY.

ACK означает принятие. Опрашивайте `Target` и `Diagnostics` до
`!Moving && !Pending`, проверяйте enabled/referenced, fault/ESTOP и
фактическую командную позицию относительно запрошенной цели с допуском
одного STEP. STOP также приводит к покою, но не доказывает достижение цели.
`PositionSteps` — выданные импульсы, а не физическая обратная связь.
`Status.EncoderAngleMdeg == math.MinInt32` означает ненастроенный энкодер.

`ZeroCommandPosition` и `ReferenceElevation` объявляют известную физическую
координату на неподвижной оси, не ищут датчик и не двигают установку.
`ZeroEncoder` изменяет только измерительный отсчёт. После ESTOP УМ теряет
привязку; CLEAR снимает fault/ESTOP, но не восстанавливает привязку.
STOP и ESTOP сохраняют уже включённое удержание; DISABLE снимает его после
замедления. После reset драйверы выключены, а координата УМ неизвестна.

`SetLinkTimeout(ctx, 0)` отключает lease; допустимы также целые 500…60000 мс.
SDK не запускает heartbeat: приложение само обслуживает обмен. По умолчанию
после reset lease=0. Истечение включённого lease вызывает STOP/fault=4.
Не блокируйте опрос длительным RX. После потери USB сравните диагностику,
конфигурацию и привязки; не возобновляйте старую цель и не обнуляйте оси
автоматически. UptimeMs оборачивается по u32.

Конфигурация хранится в RAM. Перед правкой считайте её целиком и после записи
проверьте чтением. Геометрия и invert_motor сбрасывают позицию/привязку;
скорости и настройки энкодера их сохраняют. Штатные full_steps/microsteps —
200/8, редукторы AZ=18/1, УМ=18/5, POL=1/1. Поле microsteps не переключает
DIP драйвера. Физический масштаб проверяется малым ходом на стенде.

Побайтовая совместимость с OPU Studio 3.3.0 проверяется в
`studio_protocol_test.go`: 21 запрос сравнивается с кадрами из её Python-
кодировщика (`testdata/opu_studio_3_3.json`, SHA-256 исходника сохранён в
fixture). Перед каждым новым запросом буфер парсера очищается, чтобы
незавершённый устаревший ответ не блокировал следующий обмен.
