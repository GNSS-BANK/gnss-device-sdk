// Package tx реализует потоковую передачу SC16 через UHD-совместимые USRP.
package tx

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

type writeFunc func(context.Context, io.Reader, device.TXConfig, stream.TXSettings) error

// Transmitter передаёт little-endian SC16 IQ через UHD-устройство.
type Transmitter struct {
	settings stream.TXSettings
	write    writeFunc
}

// Option настраивает Transmitter.
type Option func(*Transmitter)

// WithChannel выбирает нумеруемый с нуля UHD TX-канал.
func WithChannel(channel int) Option {
	return func(transmitter *Transmitter) {
		transmitter.settings.Channel = channel
	}
}

// WithBufferLength задаёт длину внутреннего буфера hz.tools/sdr/uhd.
func WithBufferLength(length int) Option {
	return func(transmitter *Transmitter) {
		transmitter.settings.BufferLength = length
	}
}

// WithChunkSize задаёт число комплексных отсчётов в одном блоке чтения.
func WithChunkSize(samples int) Option {
	return func(transmitter *Transmitter) {
		transmitter.settings.ChunkSamples = samples
	}
}

// WithSettleDelay задаёт задержку между настройкой устройства и стартом TX.
func WithSettleDelay(delay time.Duration) Option {
	return func(transmitter *Transmitter) {
		transmitter.settings.SettleDelay = delay
	}
}

// New создаёт UHD-передатчик.
func New(options ...Option) *Transmitter {
	transmitter := &Transmitter{
		settings: stream.TXSettings{
			Channel:      0,
			BufferLength: stream.DefaultBufferLength,
			ChunkSamples: stream.DefaultChunkSamples,
			SettleDelay:  stream.DefaultSettleDelay,
		},
		write: stream.Write,
	}
	for _, option := range options {
		if option != nil {
			option(transmitter)
		}
	}
	return transmitter
}

// SampleFormat возвращает нативный формат потока UHD SC16.
func (t *Transmitter) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt16LE
}

// Write настраивает UHD-устройство и потоково передаёт SC16 из src.
func (t *Transmitter) Write(ctx context.Context, src io.Reader, config device.TXConfig) error {
	if t == nil {
		return errors.New("UHD transmitter is nil")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	if src == nil {
		return errors.New("transmit source is required")
	}
	config.DeviceID = strings.TrimSpace(config.DeviceID)
	if err := stream.ValidateTX(config, t.settings); err != nil {
		return fmt.Errorf("invalid UHD TX config: %w", err)
	}
	if err := t.write(ctx, src, config, t.settings); err != nil {
		return fmt.Errorf("write to UHD: %w", err)
	}
	return nil
}

var _ device.Transmitter = (*Transmitter)(nil)
