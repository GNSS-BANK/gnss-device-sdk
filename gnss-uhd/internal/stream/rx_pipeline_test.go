package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	sdrlib "hz.tools/sdr"
)

type sampleReaderFunc func(sdrlib.Samples) (int, error)

func (fn sampleReaderFunc) Read(samples sdrlib.Samples) (int, error) {
	return fn(samples)
}

func TestSampleBlockPoolHasFixedCapacity(t *testing.T) {
	pool, err := newSampleBlockPool(8, 256)
	if err != nil {
		t.Fatalf("newSampleBlockPool() error = %v", err)
	}
	wantBlocks := 256 / (8 * BytesPerSample)
	if cap(pool.free) != wantBlocks || cap(pool.ready) != wantBlocks {
		t.Fatalf("pool capacities = free:%d ready:%d, want %d", cap(pool.free), cap(pool.ready), wantBlocks)
	}
	if pool.capacityBytes != 256 {
		t.Fatalf("capacityBytes = %d, want 256", pool.capacityBytes)
	}
}

func TestCaptureSamplesWritesLittleEndianSC16(t *testing.T) {
	read := false
	reader := sampleReaderFunc(func(raw sdrlib.Samples) (int, error) {
		if read {
			return 0, io.EOF
		}
		read = true
		samples := raw.(sdrlib.SamplesI16)
		samples[0] = [2]int16{1, -2}
		samples[1] = [2]int16{0x1234, -0x1234}
		return 2, io.EOF
	})
	pool, err := newSampleBlockPool(4, 32)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	total, err := captureSamplesWithPool(context.Background(), reader, &output, 10_000_000, 0, pool)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("capture error = %v, want EOF", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	want := []byte{0x01, 0x00, 0xfe, 0xff, 0x34, 0x12, 0xcc, 0xed}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("output = %x, want %x", output.Bytes(), want)
	}
}

func TestCaptureSamplesStopsAtSampleCount(t *testing.T) {
	reader := sampleReaderFunc(func(raw sdrlib.Samples) (int, error) {
		samples := raw.(sdrlib.SamplesI16)
		for index := range samples {
			samples[index] = [2]int16{int16(index + 1), -int16(index + 1)}
		}
		return len(samples), nil
	})
	pool, err := newSampleBlockPool(4, 32)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	total, err := captureSamplesWithPool(context.Background(), reader, &output, 1, 3, pool)
	if err != nil {
		t.Fatalf("capture error = %v", err)
	}
	if total != 3 || output.Len() != 3*BytesPerSample {
		t.Fatalf("total = %d, bytes = %d", total, output.Len())
	}
}

func TestCaptureSamplesPropagatesDestinationError(t *testing.T) {
	wantErr := errors.New("disk failed")
	reader := sampleReaderFunc(func(raw sdrlib.Samples) (int, error) {
		return len(raw.(sdrlib.SamplesI16)), nil
	})
	pool, err := newSampleBlockPool(4, 32)
	if err != nil {
		t.Fatal(err)
	}
	_, err = captureSamplesWithPool(context.Background(), reader, errorWriter{err: wantErr}, 1, 4, pool)
	if !errors.Is(err, wantErr) {
		t.Fatalf("capture error = %v, want %v", err, wantErr)
	}
}

type errorWriter struct{ err error }

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
