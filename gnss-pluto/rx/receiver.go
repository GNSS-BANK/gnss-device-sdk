// Package rx реализует потоковый приём данных с ADALM-Pluto.
package rx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"github.com/GNSS-BANK/gnss-device-sdk/gnss-pluto/internal/iio"
)

const (
	defaultAttrBinary = "iio_attr"
	defaultReadBinary = "iio_readdev"
	defaultBufferSize = 32_768
)

// Receiver читает комплексные IQ-отсчёты int16 little-endian с Pluto через
// официальные утилиты libiio.
type Receiver struct {
	binaries   iio.Binaries
	bufferSize uint32
	run        iio.Runner
	stderr     io.Writer
}

// Option настраивает Receiver.
type Option func(*Receiver)

// WithIIOAttrBinary переопределяет имя или путь к iio_attr.
func WithIIOAttrBinary(binary string) Option {
	return func(receiver *Receiver) {
		receiver.binaries.Attr = strings.TrimSpace(binary)
	}
}

// WithIIOReaddevBinary переопределяет имя или путь к iio_readdev.
func WithIIOReaddevBinary(binary string) Option {
	return func(receiver *Receiver) {
		receiver.binaries.Read = strings.TrimSpace(binary)
	}
}

// WithBufferSize задаёт размер IIO-буфера в комплексных отсчётах.
func WithBufferSize(size uint32) Option {
	return func(receiver *Receiver) {
		receiver.bufferSize = size
	}
}

// WithStderr направляет диагностику libiio в writer. Ограниченный по размеру
// хвост вывода также сохраняется и добавляется в ошибки процесса.
func WithStderr(writer io.Writer) Option {
	return func(receiver *Receiver) {
		receiver.stderr = writer
	}
}

// New создаёт приёмник ADALM-Pluto.
func New(options ...Option) *Receiver {
	receiver := &Receiver{
		binaries: iio.Binaries{
			Attr: defaultAttrBinary,
			Read: defaultReadBinary,
		},
		bufferSize: defaultBufferSize,
		run:        iio.Run,
	}
	for _, option := range options {
		if option != nil {
			option(receiver)
		}
	}
	return receiver
}

// SampleFormat возвращает формат I/Q-потока Pluto. RX предоставляет знаковые
// 12-битные значения в 16-битных little-endian контейнерах.
func (r *Receiver) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt16LE
}

// Read настраивает Pluto и потоково читает отсчёты в dst.
func (r *Receiver) Read(ctx context.Context, dst io.Writer, config device.RXConfig) error {
	if r == nil {
		return errors.New("Pluto receiver is nil")
	}
	if dst == nil {
		return errors.New("receive destination is required")
	}

	commands, err := iio.RXPlan(config, r.binaries, r.bufferSize)
	if err != nil {
		return fmt.Errorf("invalid Pluto RX config: %w", err)
	}
	for index, command := range commands {
		stdout := io.Writer(io.Discard)
		if index == len(commands)-1 {
			stdout = dst
		}
		if err := r.run(ctx, command.Binary, command.Args, nil, stdout, r.stderr); err != nil {
			if index == len(commands)-1 {
				return fmt.Errorf("read from Pluto: %w", err)
			}
			return fmt.Errorf("configure Pluto RX: %w", err)
		}
	}
	return nil
}

var _ device.Receiver = (*Receiver)(nil)
