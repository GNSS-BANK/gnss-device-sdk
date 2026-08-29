// Package rx реализует потоковый приём SC16 с UHD-совместимых USRP.
package rx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-uhd/internal/stream"
)

type readFunc func(context.Context, io.Writer, device.RXConfig, stream.RXSettings) error

// Receiver читает little-endian SC16 IQ с UHD-устройства.
type Receiver struct {
	settings stream.RXSettings
	read     readFunc
}

// Option настраивает Receiver.
type Option func(*Receiver)

// WithChannel выбирает нумеруемый с нуля UHD RX-канал.
func WithChannel(channel int) Option {
	return func(receiver *Receiver) {
		receiver.settings.Channel = channel
	}
}

// WithBufferLength задаёт длину внутреннего буфера hz.tools/sdr/uhd.
func WithBufferLength(length int) Option {
	return func(receiver *Receiver) {
		receiver.settings.BufferLength = length
	}
}

// WithChunkSize задаёт число комплексных отсчётов в одном блоке записи.
func WithChunkSize(samples int) Option {
	return func(receiver *Receiver) {
		receiver.settings.ChunkSamples = samples
	}
}

// WithRXBufferSize задаёт общий объём переиспользуемого RX-буфера в байтах.
func WithRXBufferSize(bytes int) Option {
	return func(receiver *Receiver) {
		receiver.settings.BufferBytes = bytes
	}
}

// WithAutomaticGain явно включает или отключает UHD AGC. Без этой option
// текущее состояние AGC устройства не изменяется.
func WithAutomaticGain(enabled bool) Option {
	return func(receiver *Receiver) {
		value := enabled
		receiver.settings.AutomaticGain = &value
	}
}

// WithSettleDelay задаёт задержку между настройкой устройства и стартом RX.
func WithSettleDelay(delay time.Duration) Option {
	return func(receiver *Receiver) {
		receiver.settings.SettleDelay = delay
	}
}

// WithRestartDelay задаёт паузу перед повторным стартом RX, если первый поток
// завершился восстанавливаемой ошибкой до получения хотя бы одного отсчёта.
func WithRestartDelay(delay time.Duration) Option {
	return func(receiver *Receiver) {
		receiver.settings.RestartDelay = delay
	}
}

// New создаёт UHD-приёмник.
func New(options ...Option) *Receiver {
	receiver := &Receiver{
		settings: stream.RXSettings{
			Channel:       0,
			BufferLength:  stream.DefaultBufferLength,
			ChunkSamples:  stream.DefaultChunkSamples,
			BufferBytes:   stream.DefaultRXBufferBytes,
			SettleDelay:   stream.DefaultSettleDelay,
			RestartDelay:  stream.DefaultRestartDelay,
			AutomaticGain: nil,
		},
		read: stream.Read,
	}
	for _, option := range options {
		if option != nil {
			option(receiver)
		}
	}
	return receiver
}

// SampleFormat возвращает нативный формат потока UHD SC16.
func (r *Receiver) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt16LE
}

// Read настраивает UHD-устройство и потоково читает SC16 в dst.
func (r *Receiver) Read(ctx context.Context, dst io.Writer, config device.RXConfig) error {
	if r == nil {
		return errors.New("UHD receiver is nil")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	if dst == nil {
		return errors.New("receive destination is required")
	}
	config.DeviceID = strings.TrimSpace(config.DeviceID)
	if err := stream.ValidateRX(config, r.settings); err != nil {
		return fmt.Errorf("invalid UHD RX config: %w", err)
	}
	if err := r.read(ctx, dst, config, r.settings); err != nil {
		return fmt.Errorf("read from UHD: %w", err)
	}
	return nil
}

var _ device.Receiver = (*Receiver)(nil)
