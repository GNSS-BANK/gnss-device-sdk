//go:build linux && cgo

package stream

import (
	"context"
	"fmt"
	"io"
	"strings"

	device "github.com/GNSS-BANK/gnss-device-sdk"
	"hz.tools/rf"
	sdrlib "hz.tools/sdr"
	"hz.tools/sdr/uhd"
)

// Write открывает UHD-устройство и передаёт в него SC16-поток из source.
func Write(ctx context.Context, source io.Reader, config device.TXConfig, settings TXSettings) error {
	conn, err := uhd.Open(uhd.Options{
		Args:         strings.TrimSpace(config.DeviceID),
		TxChannel:    settings.Channel,
		SampleFormat: sdrlib.SampleFormatI16,
		BufferLength: settings.BufferLength,
	})
	if err != nil {
		return fmt.Errorf("failed to open UHD device: %w", err)
	}

	if err := conn.SetSampleRate(uint(config.SampleRateHz)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to set UHD sample rate: %w", err)
	}
	if err := conn.SetCenterFrequencyTX(rf.Hz(config.CenterFrequencyHz)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to set UHD TX center frequency: %w", err)
	}
	if err := applyGains(conn, config.Gains, sdrlib.GainStageTypeTransmit); err != nil {
		_ = conn.Close()
		return err
	}
	if err := sleepWithContext(ctx, settings.SettleDelay); err != nil {
		_ = conn.Close()
		return err
	}

	tx, err := conn.StartTx()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to start UHD TX: %w", err)
	}
	_, streamErr := transmitSamples(ctx, source, tx, settings.ChunkSamples, config.SampleCount)
	closeStreamErr := tx.Close()
	closeDeviceErr := conn.Close()
	if streamErr != nil {
		return streamErr
	}
	if closeStreamErr != nil {
		return fmt.Errorf("failed to close UHD TX stream: %w", closeStreamErr)
	}
	if closeDeviceErr != nil {
		return fmt.Errorf("failed to close UHD device: %w", closeDeviceErr)
	}
	return nil
}
