// Package tx реализует потоковую передачу данных через ADALM-Pluto.
package tx

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
	defaultAttrBinary  = "iio_attr"
	defaultWriteBinary = "iio_writedev"
	defaultBufferSize  = 32_768
)

// Transmitter передаёт комплексные IQ-отсчёты int16 little-endian в Pluto
// через официальные утилиты libiio.
type Transmitter struct {
	binaries   iio.Binaries
	bufferSize uint32
	run        iio.Runner
	stderr     io.Writer
}

// Option настраивает Transmitter.
type Option func(*Transmitter)

// WithIIOAttrBinary переопределяет имя или путь к iio_attr.
func WithIIOAttrBinary(binary string) Option {
	return func(transmitter *Transmitter) {
		transmitter.binaries.Attr = strings.TrimSpace(binary)
	}
}

// WithIIOWritedevBinary переопределяет имя или путь к iio_writedev.
func WithIIOWritedevBinary(binary string) Option {
	return func(transmitter *Transmitter) {
		transmitter.binaries.Write = strings.TrimSpace(binary)
	}
}

// WithBufferSize задаёт размер IIO-буфера в комплексных отсчётах.
func WithBufferSize(size uint32) Option {
	return func(transmitter *Transmitter) {
		transmitter.bufferSize = size
	}
}

// WithStderr направляет диагностику libiio в writer. Ограниченный по размеру
// хвост вывода также сохраняется и добавляется в ошибки процесса.
func WithStderr(writer io.Writer) Option {
	return func(transmitter *Transmitter) {
		transmitter.stderr = writer
	}
}

// New создаёт передатчик ADALM-Pluto.
func New(options ...Option) *Transmitter {
	transmitter := &Transmitter{
		binaries: iio.Binaries{
			Attr:  defaultAttrBinary,
			Write: defaultWriteBinary,
		},
		bufferSize: defaultBufferSize,
		run:        iio.Run,
	}
	for _, option := range options {
		if option != nil {
			option(transmitter)
		}
	}
	return transmitter
}

// SampleFormat возвращает формат I/Q-потока Pluto.
func (t *Transmitter) SampleFormat() device.SampleFormat {
	return device.SampleFormatComplexInt16LE
}

// Write настраивает Pluto и потоково передаёт отсчёты из src.
func (t *Transmitter) Write(ctx context.Context, src io.Reader, config device.TXConfig) error {
	if t == nil {
		return errors.New("Pluto transmitter is nil")
	}
	if src == nil {
		return errors.New("transmit source is required")
	}

	commands, err := iio.TXPlan(config, t.binaries, t.bufferSize)
	if err != nil {
		return fmt.Errorf("invalid Pluto TX config: %w", err)
	}
	for index, command := range commands {
		stdin := io.Reader(nil)
		if index == len(commands)-1 {
			stdin = src
		}
		if err := t.run(ctx, command.Binary, command.Args, stdin, io.Discard, t.stderr); err != nil {
			if index == len(commands)-1 {
				return fmt.Errorf("write to Pluto: %w", err)
			}
			return fmt.Errorf("configure Pluto TX: %w", err)
		}
	}
	return nil
}

var _ device.Transmitter = (*Transmitter)(nil)
