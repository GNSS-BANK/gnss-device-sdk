package stream

import "time"

const (
	BytesPerSample       = 4
	DefaultBufferLength  = 10
	DefaultChunkSamples  = 1 << 18
	DefaultRXBufferBytes = 256 << 20
	DefaultSettleDelay   = 250 * time.Millisecond
	DefaultRestartDelay  = 500 * time.Millisecond
	maximumSampleCount   = uint64(1 << 63)
)

// RXSettings содержит специфичные для UHD параметры приёмника.
type RXSettings struct {
	Channel       int
	BufferLength  int
	ChunkSamples  int
	BufferBytes   int
	AutomaticGain *bool
	SettleDelay   time.Duration
	RestartDelay  time.Duration
}

// TXSettings содержит специфичные для UHD параметры передатчика.
type TXSettings struct {
	Channel      int
	BufferLength int
	ChunkSamples int
	SettleDelay  time.Duration
}
