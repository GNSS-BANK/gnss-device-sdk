package stream

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	sdrlib "hz.tools/sdr"
)

type sampleWriter interface {
	Write(sdrlib.Samples) (int, error)
}

func transmitSamples(
	ctx context.Context,
	source io.Reader,
	writer sampleWriter,
	chunkSamples int,
	sampleCount uint64,
) (uint64, error) {
	samples := make(sdrlib.SamplesI16, chunkSamples)
	raw := make([]byte, chunkSamples*BytesPerSample)
	totalSamples := uint64(0)

	for sampleCount == 0 || totalSamples < sampleCount {
		if err := ctx.Err(); err != nil {
			return totalSamples, err
		}

		target := chunkSamples
		if sampleCount != 0 {
			remaining := sampleCount - totalSamples
			if remaining < uint64(target) {
				target = int(remaining)
			}
		}

		bytesRead, readErr := io.ReadFull(source, raw[:target*BytesPerSample])
		if readErr == io.EOF && bytesRead == 0 {
			return totalSamples, nil
		}
		if bytesRead%BytesPerSample != 0 {
			return totalSamples, fmt.Errorf("UHD TX input ends inside an SC16 sample: %d trailing bytes", bytesRead%BytesPerSample)
		}
		count := bytesRead / BytesPerSample
		if count > 0 {
			if _, err := binary.Decode(raw[:bytesRead], binary.LittleEndian, samples[:count]); err != nil {
				return totalSamples, fmt.Errorf("failed to decode UHD TX samples: %w", err)
			}
			written, err := writer.Write(samples[:count])
			if err != nil {
				return totalSamples, fmt.Errorf("failed to write UHD TX samples: %w", err)
			}
			if written != count {
				return totalSamples, fmt.Errorf("failed to write UHD TX samples: %w: wrote %d of %d samples", io.ErrShortWrite, written, count)
			}
			totalSamples += uint64(count)
		}
		if readErr == io.ErrUnexpectedEOF {
			return totalSamples, nil
		}
		if readErr != nil {
			return totalSamples, fmt.Errorf("failed to read UHD TX input: %w", readErr)
		}
	}
	return totalSamples, nil
}
