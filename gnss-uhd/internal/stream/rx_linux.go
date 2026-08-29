//go:build linux && cgo

package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"hz.tools/rf"
	sdrlib "hz.tools/sdr"
	"hz.tools/sdr/uhd"
)

// Read открывает UHD-устройство и передаёт его SC16-поток в destination.
func Read(ctx context.Context, destination io.Writer, config device.RXConfig, settings RXSettings) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pool, err := newSampleBlockPool(settings.ChunkSamples, settings.BufferBytes)
	if err != nil {
		return err
	}

	var lastRecoverable error
	for {
		if err := ctx.Err(); err != nil {
			if lastRecoverable != nil {
				return fmt.Errorf("no UHD samples captured before stop: %w", lastRecoverable)
			}
			return err
		}

		conn, err := openRX(ctx, config, settings)
		if err != nil {
			return err
		}
		captured, streamErr := captureRX(ctx, conn, destination, config, settings, pool)
		closeErr := conn.Close()
		if streamErr == nil {
			if closeErr != nil {
				return fmt.Errorf("failed to close UHD device: %w", closeErr)
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return nil
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close UHD device after stream error: %w", closeErr)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if captured > 0 {
			return fmt.Errorf("UHD stream interrupted after %d samples: %w", captured, streamErr)
		}
		if !isRecoverableReadError(streamErr) {
			return fmt.Errorf("failed to read UHD samples: %w", streamErr)
		}

		lastRecoverable = streamErr
		if err := sleepWithContext(ctx, settings.RestartDelay); err != nil {
			return fmt.Errorf("no UHD samples captured before stop: %w", lastRecoverable)
		}
		if err := pool.resetForNextCapture(); err != nil {
			return err
		}
	}
}

func openRX(ctx context.Context, config device.RXConfig, settings RXSettings) (*uhd.Sdr, error) {
	conn, err := uhd.Open(uhd.Options{
		Args:         strings.TrimSpace(config.DeviceID),
		RxChannel:    settings.Channel,
		SampleFormat: sdrlib.SampleFormatI16,
		BufferLength: settings.BufferLength,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open UHD device: %w", err)
	}

	if err := conn.SetSampleRate(uint(config.SampleRateHz)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set UHD sample rate: %w", err)
	}
	if err := conn.SetCenterFrequencyRX(rf.Hz(config.CenterFrequencyHz)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set UHD RX center frequency: %w", err)
	}
	if settings.AutomaticGain != nil {
		if err := conn.SetAutomaticGain(*settings.AutomaticGain); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to set UHD automatic gain: %w", err)
		}
	}
	if err := applyGains(conn, config.Gains, sdrlib.GainStageTypeRecieve); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := sleepWithContext(ctx, settings.SettleDelay); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func captureRX(
	ctx context.Context,
	conn *uhd.Sdr,
	destination io.Writer,
	config device.RXConfig,
	settings RXSettings,
	pool *sampleBlockPool,
) (uint64, error) {
	rx, err := conn.StartRx()
	if err != nil {
		return 0, fmt.Errorf("failed to start UHD RX: %w", err)
	}
	captured, streamErr := captureSamplesWithPool(ctx, rx, destination, config.SampleRateHz, config.SampleCount, pool)
	closeErr := rx.Close()
	if streamErr != nil {
		return captured, streamErr
	}
	if closeErr != nil {
		return captured, fmt.Errorf("failed to close UHD RX stream: %w", closeErr)
	}
	return captured, nil
}

func isRecoverableReadError(err error) bool {
	return errors.Is(err, uhd.ErrRxMetadataTimeout) ||
		errors.Is(err, uhd.ErrRxMetadataOverflow) ||
		errors.Is(err, sdrlib.ErrIO) ||
		errors.Is(err, sdrlib.ErrUSB) ||
		errors.Is(err, sdrlib.ErrOS) ||
		errors.Is(err, sdrlib.ErrRuntime) ||
		errors.Is(err, io.EOF)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
