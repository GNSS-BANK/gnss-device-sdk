// Package device определяет не зависящие от оборудования потоковые контракты
// для радиоустройств GNSS.
package device

import (
	"context"
	"io"
)

// SampleFormat описывает двоичное представление одного комплексного IQ-отсчёта.
type SampleFormat string

const (
	// SampleFormatComplexInt8 — чередующиеся знаковые 8-битные I/Q: I0, Q0, I1, Q1.
	SampleFormatComplexInt8 SampleFormat = "complex-int8"
)

// Gain задаёт значение именованного каскада усиления в децибелах.
//
// Названия каскадов и допустимые значения проверяет адаптер конкретного устройства.
type Gain struct {
	Stage   string
	ValueDB float64
}

// StreamConfig содержит общие настройки операций приёма и передачи.
// Нулевой BandwidthHz позволяет адаптеру выбрать полосу по умолчанию. Нулевой
// SampleCount означает, что поток продолжается до остановки источника или
// отмены контекста.
type StreamConfig struct {
	DeviceID          string
	CenterFrequencyHz uint64
	SampleRateHz      uint32
	BandwidthHz       uint32
	SampleCount       uint64
}

// RXConfig задаёт параметры операции приёма.
type RXConfig struct {
	StreamConfig
	Gains               []Gain
	RFAmplifierEnabled  bool
	AntennaPowerEnabled bool
	HardwareTrigger     bool
}

// TXConfig задаёт параметры операции передачи.
type TXConfig struct {
	StreamConfig
	Gains               []Gain
	RFAmplifierEnabled  bool
	AntennaPowerEnabled bool
	HardwareTrigger     bool
}

// Receiver читает отсчёты устройства в dst до достижения SampleCount, отмены
// контекста или остановки потока устройством.
type Receiver interface {
	SampleFormat() SampleFormat
	Read(ctx context.Context, dst io.Writer, config RXConfig) error
}

// Transmitter передаёт отсчёты из src в устройство до достижения SampleCount,
// окончания входного потока или отмены контекста.
type Transmitter interface {
	SampleFormat() SampleFormat
	Write(ctx context.Context, src io.Reader, config TXConfig) error
}
