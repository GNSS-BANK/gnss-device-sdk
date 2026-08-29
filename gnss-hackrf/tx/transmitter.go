// Package tx реализует потоковую передачу данных через HackRF.
package tx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-hackrf/internal/transfer"
)

const defaultBinary = "hackrf_transfer"

// Transmitter передаёт комплексные IQ-отсчёты int8 в HackRF через
// hackrf_transfer.
type Transmitter struct {
	binary string
	run    transfer.Runner
	stderr io.Writer
}

// Option настраивает Transmitter.
type Option func(*Transmitter)

// WithBinary переопределяет имя или путь к исполняемому файлу hackrf_transfer.
func WithBinary(binary string) Option {
	return func(transmitter *Transmitter) {
		transmitter.binary = strings.TrimSpace(binary)
	}
}

// WithStderr направляет диагностику hackrf_transfer в writer. Ограниченный по
// размеру хвост вывода также сохраняется и добавляется в ошибки процесса.
func WithStderr(writer io.Writer) Option {
	return func(transmitter *Transmitter) {
		transmitter.stderr = writer
	}
}

// New создаёт передатчик HackRF.
func New(options ...Option) *Transmitter {
	transmitter := &Transmitter{
		binary: defaultBinary,
		run:    transfer.Run,
	}
	for _, option := range options {
		if option != nil {
			option(transmitter)
		}
	}
	return transmitter
}

// SampleFormat возвращает нативный формат передачи отсчётов HackRF.
func (t *Transmitter) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt8
}

// Write потоково передаёт отсчёты из src в HackRF.
func (t *Transmitter) Write(ctx context.Context, src io.Reader, config device.TXConfig) error {
	if t == nil {
		return errors.New("HackRF transmitter is nil")
	}
	if src == nil {
		return errors.New("transmit source is required")
	}

	args, err := transfer.TXArgs(config)
	if err != nil {
		return fmt.Errorf("invalid HackRF TX config: %w", err)
	}
	if err := t.run(ctx, t.binary, args, src, io.Discard, t.stderr); err != nil {
		return fmt.Errorf("write to HackRF: %w", err)
	}
	return nil
}

var _ device.Transmitter = (*Transmitter)(nil)
