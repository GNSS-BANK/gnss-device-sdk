package stm32

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GNSS-BANK/gnss-device-sdk/opu"
)

func TestFirmware3DocumentVectors(t *testing.T) {
	for _, hex := range []string{
		"A5 5A 01 02 01 00 00 00 87 79",
		"A5 5A 01 82 01 00 11 00 00 03 00 01 03 20 4E 00 00 88 13 00 00 EF 1F 00 00 BE EF",
		"A5 5A 01 20 02 00 01 00 01 35 1F",
		"A5 5A 01 32 03 00 01 00 01 A0 24",
		"A5 5A 01 24 04 00 05 00 01 88 13 00 00 AF 95",
		"A5 5A 01 24 05 00 05 00 00 30 75 00 00 04 89",
		"A5 5A 01 44 09 00 05 00 01 58 9E FF FF 8C 4C",
		"A5 5A 01 43 0A 00 08 00 58 9E FF FF 32 56 4C 45 FC 18",
		"A5 5A 01 25 08 00 01 00 FF 4B 14",
		"A5 5A 01 A4 04 00 01 00 03 73 20",
	} {
		frame := mustHex(hex)
		var p frameParser
		frames := p.feed(frame)
		if len(frames) != 1 {
			t.Fatalf("invalid document vector: %s", hex)
		}
		got, err := encodeFrame(frames[0].cmd, frames[0].seq, frames[0].payload)
		if err != nil || !bytes.Equal(got, frame) {
			t.Fatalf("vector mismatch: %x, %v", got, err)
		}
	}
}

func TestFirmware3CommandsAndTelemetry(t *testing.T) {
	target := mustHex("01 01 01 01 70 FE FF FF FF FF FF FF A0 05 00 00 00 00 00 00 58 9E FF FF 90 5F 01 00 01 03 00 00")
	diag := mustHex("70 5E 40 DF 01 00 00 00 78 56 34 12 02 00 00 00 03 00 00 00 04 00 00 00 05 00 00 00 B8 0B 00 00 04 00 07 01")
	p := &fakePort{answer: func(req []byte) []byte {
		switch req[3] {
		case cmdGetTarget:
			return respond(req, 0, target)
		case cmdDiagnostics:
			return respond(req, 0, diag)
		default:
			return respond(req, 0, nil)
		}
	}}
	c, err := New(p, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	got, err := c.Target(ctx, opu.Elevation)
	if err != nil || got.Axis != opu.Elevation || !got.Moving || !got.Enabled || !got.Referenced ||
		got.PositionSteps != -400 || got.TargetSteps != 1440 || got.PositionMdeg != -25000 ||
		got.TargetMdeg != 90000 || !got.Pending || !got.InvertMotor || !got.InvertEncoder {
		t.Fatalf("target: %+v, %v", got, err)
	}
	d, err := c.Diagnostics(ctx)
	if err != nil || d.BuildID != 0xdf405e70 || d.ResetFlags != 1 || d.UptimeMs != 0x12345678 ||
		d.USBSessions != 2 || d.TXTimeouts != 3 || d.RXOverflows != 4 || d.CRCErrors != 5 ||
		d.LinkTimeoutMs != 3000 || d.Fault != opu.ElevationFaultLink || d.EmergencyStopped || d.ReferenceMask != 7 || !d.USBOnline {
		t.Fatalf("diagnostics: %+v, %v", d, err)
	}
	if err := c.MoveAbsoluteUnwrapped(ctx, opu.Elevation, -25000); err != nil {
		t.Fatal(err)
	}
	last := p.writes[len(p.writes)-1]
	if last[3] != 0x44 || !bytes.Equal(last[8:13], mustHex("01 58 9E FF FF")) {
		t.Fatalf("unwrapped: %x", last)
	}
	for _, lease := range []time.Duration{0, 500 * time.Millisecond, 3 * time.Second, 60 * time.Second} {
		if err := c.SetLinkTimeout(ctx, lease); err != nil {
			t.Fatal(err)
		}
		last = p.writes[len(p.writes)-1]
		if last[3] != 0x46 || binary.LittleEndian.Uint32(last[8:12]) != uint32(lease/time.Millisecond) {
			t.Fatalf("lease: %x", last)
		}
	}
	count := len(p.writes)
	for _, lease := range []time.Duration{-time.Millisecond, 499 * time.Millisecond, 60001 * time.Millisecond, 500*time.Millisecond + 1} {
		if err := c.SetLinkTimeout(ctx, lease); err == nil {
			t.Fatalf("invalid lease accepted: %s", lease)
		}
	}
	if _, err := c.Target(ctx, opu.AllAxes); err == nil {
		t.Fatal("accepted invalid axis")
	}
	if err := c.MoveAbsoluteUnwrapped(ctx, opu.Azimuth, 36000001); err == nil {
		t.Fatal("accepted invalid angle")
	}
	if len(p.writes) != count {
		t.Fatal("invalid arguments reached transport")
	}
	p.answer = func(req []byte) []byte { return respond(req, 17, nil) }
	err = c.MoveAbsoluteUnwrapped(ctx, opu.Azimuth, 270000)
	var result *ResultError
	if !errors.As(err, &result) || result.Code != ResultSequenceConflict || !strings.Contains(err.Error(), "SEQUENCE_CONFLICT") {
		t.Fatalf("sequence conflict: %v", err)
	}
}

func TestFirmware3RejectsMalformedTelemetry(t *testing.T) {
	target := make([]byte, 32)
	for _, offset := range []int{0, 1, 2, 3, 28, 29, 30, 31} {
		bad := bytes.Clone(target)
		bad[offset] = 0xff
		if _, err := parseTarget(bad); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted target offset %d", offset)
		}
	}
	for _, n := range []int{0, 31, 33} {
		if _, err := parseTarget(make([]byte, n)); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted target length %d", n)
		}
	}
	for _, offset := range []int{32, 33, 34, 35} {
		bad := make([]byte, 36)
		bad[offset] = 0xff
		if _, err := parseDiagnostics(bad); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted diagnostic offset %d", offset)
		}
	}
	for _, n := range []int{0, 35, 37} {
		if _, err := parseDiagnostics(make([]byte, n)); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted diagnostic length %d", n)
		}
	}
	p := &fakePort{answer: func(req []byte) []byte { return respond(req, 0, target) }}
	c, _ := New(p, time.Second)
	if _, err := c.Target(context.Background(), opu.Elevation); !errors.Is(err, ErrProtocol) {
		t.Fatalf("accepted wrong axis: %v", err)
	}
}

func TestUnwrappedTimeoutDoesNotRetryMotion(t *testing.T) {
	p := &fakePort{}
	c, _ := New(p, 5*time.Millisecond)
	if err := c.MoveAbsoluteUnwrapped(context.Background(), opu.Azimuth, 360000); err == nil || len(p.writes) != 1 {
		t.Fatalf("timeout sent %d requests: %v", len(p.writes), err)
	}
}
