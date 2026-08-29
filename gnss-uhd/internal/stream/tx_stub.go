//go:build !linux || !cgo

package stream

import (
	"context"
	"errors"
	"io"

	device "github.com/GNSS-BANK/gnss-device-sdk"
)

// Write сообщает о недоступности UHD вне Linux/CGO-сборки.
func Write(context.Context, io.Reader, device.TXConfig, TXSettings) error {
	return errors.New("UHD TX requires a linux/cgo build with installed UHD libraries")
}
