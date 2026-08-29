package stream

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	sdrlib "hz.tools/sdr"
)

type collectingSampleWriter struct {
	samples sdrlib.SamplesI16
	err     error
	short   bool
}

func (writer *collectingSampleWriter) Write(raw sdrlib.Samples) (int, error) {
	samples := raw.(sdrlib.SamplesI16)
	writer.samples = append(writer.samples, samples...)
	if writer.err != nil {
		return 0, writer.err
	}
	if writer.short {
		return len(samples) - 1, nil
	}
	return len(samples), nil
}

func TestTransmitSamplesReadsLittleEndianSC16(t *testing.T) {
	input := []byte{
		0x01, 0x00, 0xfe, 0xff,
		0x34, 0x12, 0xcc, 0xed,
	}
	writer := &collectingSampleWriter{}
	total, err := transmitSamples(context.Background(), bytes.NewReader(input), writer, 4, 0)
	if err != nil {
		t.Fatalf("transmitSamples() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	want := sdrlib.SamplesI16{{1, -2}, {0x1234, -0x1234}}
	if !reflect.DeepEqual(writer.samples, want) {
		t.Fatalf("samples = %#v, want %#v", writer.samples, want)
	}
}

func TestTransmitSamplesHonorsSampleCount(t *testing.T) {
	input := make([]byte, 3*BytesPerSample)
	writer := &collectingSampleWriter{}
	total, err := transmitSamples(context.Background(), bytes.NewReader(input), writer, 4, 2)
	if err != nil {
		t.Fatalf("transmitSamples() error = %v", err)
	}
	if total != 2 || len(writer.samples) != 2 {
		t.Fatalf("total = %d, samples = %d", total, len(writer.samples))
	}
}

func TestTransmitSamplesRejectsPartialSample(t *testing.T) {
	_, err := transmitSamples(context.Background(), bytes.NewReader([]byte{1, 2, 3}), &collectingSampleWriter{}, 4, 0)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("inside an SC16 sample")) {
		t.Fatalf("transmitSamples() error = %v", err)
	}
}

func TestTransmitSamplesPropagatesWriterError(t *testing.T) {
	wantErr := errors.New("UHD failed")
	_, err := transmitSamples(context.Background(), bytes.NewReader(make([]byte, BytesPerSample)), &collectingSampleWriter{err: wantErr}, 4, 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("transmitSamples() error = %v, want %v", err, wantErr)
	}
}
