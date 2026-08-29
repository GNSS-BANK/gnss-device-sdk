package iio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

const errorOutputLimit = 64 * 1024

// Runner запускает одну утилиту libiio с потоковыми stdin/stdout.
type Runner func(
	ctx context.Context,
	binary string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error

// Run запускает утилиту libiio и добавляет ограниченный хвост stderr в ошибку.
func Run(
	ctx context.Context,
	binary string,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if strings.TrimSpace(binary) == "" {
		return errors.New("libiio binary is required")
	}

	command := exec.CommandContext(ctx, binary, args...)
	command.Stdin = stdin
	command.Stdout = stdout

	errorOutput := &tailBuffer{limit: errorOutputLimit}
	command.Stderr = errorOutput
	if stderr != nil {
		command.Stderr = io.MultiWriter(stderr, errorOutput)
	}

	if err := command.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		name := filepath.Base(binary)
		detail := strings.TrimSpace(errorOutput.String())
		if detail != "" {
			return fmt.Errorf("%s failed: %w: %s", name, err, detail)
		}
		return fmt.Errorf("%s failed: %w", name, err)
	}

	return nil
}

type tailBuffer struct {
	data  []byte
	limit int
}

func (b *tailBuffer) Write(payload []byte) (int, error) {
	written := len(payload)
	if b.limit <= 0 || written == 0 {
		return written, nil
	}
	if written >= b.limit {
		b.data = append(b.data[:0], payload[written-b.limit:]...)
		return written, nil
	}

	overflow := len(b.data) + written - b.limit
	if overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
	}
	b.data = append(b.data, payload...)
	return written, nil
}

func (b *tailBuffer) String() string {
	return string(bytes.Clone(b.data))
}
