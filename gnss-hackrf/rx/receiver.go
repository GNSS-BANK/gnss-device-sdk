// Package rx реализует потоковый приём данных с HackRF.
package rx

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

// Receiver читает комплексные IQ-отсчёты int8 с HackRF через hackrf_transfer.
type Receiver struct {
	binary string
	run    transfer.Runner
	stderr io.Writer
}

// Option настраивает Receiver.
type Option func(*Receiver)

// WithBinary переопределяет имя или путь к исполняемому файлу hackrf_transfer.
func WithBinary(binary string) Option {
	return func(receiver *Receiver) {
		receiver.binary = strings.TrimSpace(binary)
	}
}

// WithStderr направляет диагностику hackrf_transfer в writer. Ограниченный по
// размеру хвост вывода также сохраняется и добавляется в ошибки процесса.
func WithStderr(writer io.Writer) Option {
	return func(receiver *Receiver) {
		receiver.stderr = writer
	}
}

// New создаёт приёмник HackRF.
func New(options ...Option) *Receiver {
	receiver := &Receiver{
		binary: defaultBinary,
		run:    transfer.Run,
	}
	for _, option := range options {
		if option != nil {
			option(receiver)
		}
	}
	return receiver
}

// SampleFormat возвращает нативный формат передачи отсчётов HackRF.
func (r *Receiver) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt8
}

// Read потоково читает отсчёты HackRF в dst.
func (r *Receiver) Read(ctx context.Context, dst io.Writer, config device.RXConfig) error {
	if r == nil {
		return errors.New("HackRF receiver is nil")
	}
	if dst == nil {
		return errors.New("receive destination is required")
	}

	args, err := transfer.RXArgs(config)
	if err != nil {
		return fmt.Errorf("invalid HackRF RX config: %w", err)
	}
	if err := r.run(ctx, r.binary, args, nil, dst, r.stderr); err != nil {
		return fmt.Errorf("read from HackRF: %w", err)
	}
	return nil
}

var _ device.Receiver = (*Receiver)(nil)
