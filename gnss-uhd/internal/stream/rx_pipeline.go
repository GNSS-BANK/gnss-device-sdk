package stream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	sdrlib "hz.tools/sdr"
)

var errCaptureBufferFull = errors.New("UHD capture buffer is full; destination cannot keep up with the receive stream")

type capturedSampleBlock struct {
	count   int
	samples sdrlib.SamplesI16
}

type sampleReader interface {
	Read(sdrlib.Samples) (int, error)
}

type sampleBlockPool struct {
	capacityBytes int
	free          chan sdrlib.SamplesI16
	ready         chan capturedSampleBlock
}

func newSampleBlockPool(chunkSamples int, bufferBytes int) (*sampleBlockPool, error) {
	if chunkSamples <= 0 {
		return nil, errors.New("capture chunk size must be > 0")
	}
	if bufferBytes <= 0 {
		return nil, errors.New("capture buffer size must be > 0")
	}
	if chunkSamples > int(^uint(0)>>1)/BytesPerSample {
		return nil, errors.New("capture chunk size is too large")
	}

	blockBytes := chunkSamples * BytesPerSample
	blockCount := bufferBytes / blockBytes
	if blockCount < 2 {
		return nil, fmt.Errorf("capture buffer must fit at least two chunks: buffer=%d bytes chunk=%d bytes", bufferBytes, blockBytes)
	}

	backing := make(sdrlib.SamplesI16, blockCount*chunkSamples)
	pool := &sampleBlockPool{
		capacityBytes: len(backing) * BytesPerSample,
		free:          make(chan sdrlib.SamplesI16, blockCount),
		ready:         make(chan capturedSampleBlock, blockCount),
	}
	for index := 0; index < blockCount; index++ {
		start := index * chunkSamples
		pool.free <- backing[start : start+chunkSamples]
	}
	return pool, nil
}

func (pool *sampleBlockPool) resetForNextCapture() error {
	if len(pool.free) != cap(pool.free) {
		return fmt.Errorf("cannot reuse capture buffer: only %d of %d blocks were returned", len(pool.free), cap(pool.free))
	}
	pool.ready = make(chan capturedSampleBlock, cap(pool.ready))
	return nil
}

func captureSamplesWithPool(
	ctx context.Context,
	reader sampleReader,
	writer io.Writer,
	sampleRate uint32,
	sampleCount uint64,
	pool *sampleBlockPool,
) (uint64, error) {
	sampleWriter := sdrlib.ByteWriter(writer, binary.LittleEndian, uint(sampleRate), sdrlib.SampleFormatI16)
	writerDone := make(chan error, 1)
	go func() {
		for block := range pool.ready {
			writtenSamples, writeErr := sampleWriter.Write(block.samples[:block.count])
			if writeErr != nil {
				writerDone <- fmt.Errorf("failed to write UHD samples: %w", writeErr)
				return
			}
			if writtenSamples != block.count {
				writerDone <- fmt.Errorf("failed to write UHD samples: %w: wrote %d of %d samples", io.ErrShortWrite, writtenSamples, block.count)
				return
			}
			pool.free <- block.samples
		}
		writerDone <- nil
	}()

	totalSamples := uint64(0)
	finish := func(streamErr error) (uint64, error) {
		close(pool.ready)
		if writeErr := <-writerDone; writeErr != nil {
			return totalSamples, writeErr
		}
		return totalSamples, streamErr
	}

	for {
		if sampleCount != 0 && totalSamples >= sampleCount {
			return finish(nil)
		}
		select {
		case writeErr := <-writerDone:
			close(pool.ready)
			return totalSamples, writeErr
		case <-ctx.Done():
			return finish(nil)
		default:
		}

		var samples sdrlib.SamplesI16
		select {
		case samples = <-pool.free:
		case writeErr := <-writerDone:
			close(pool.ready)
			return totalSamples, writeErr
		case <-ctx.Done():
			return finish(nil)
		default:
			return finish(errCaptureBufferFull)
		}

		target := len(samples)
		if sampleCount != 0 {
			remaining := sampleCount - totalSamples
			if remaining < uint64(target) {
				target = int(remaining)
			}
		}

		count := 0
		for count < target {
			select {
			case writeErr := <-writerDone:
				pool.free <- samples
				close(pool.ready)
				return totalSamples, writeErr
			case <-ctx.Done():
				if count > 0 {
					pool.ready <- capturedSampleBlock{count: count, samples: samples}
				} else {
					pool.free <- samples
				}
				return finish(nil)
			default:
			}

			n, readErr := reader.Read(samples[count:target])
			if n < 0 || n > target-count {
				pool.free <- samples
				return finish(fmt.Errorf("UHD reader returned invalid sample count %d for remaining chunk size %d", n, target-count))
			}
			count += n
			totalSamples += uint64(n)
			if readErr != nil {
				if count > 0 {
					pool.ready <- capturedSampleBlock{count: count, samples: samples}
				} else {
					pool.free <- samples
				}
				return finish(readErr)
			}
		}

		pool.ready <- capturedSampleBlock{count: count, samples: samples}
	}
}
