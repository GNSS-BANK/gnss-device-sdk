//go:build !linux || !cgo

package stream

import (
	"context"
	"errors"
	"io"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

// Read сообщает о недоступности UHD вне Linux/CGO-сборки.
func Read(context.Context, io.Writer, device.RXConfig, RXSettings) error {
	return errors.New("UHD RX requires a linux/cgo build with installed UHD libraries")
}
