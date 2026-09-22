package stm32

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GNSS-BANK/gnss-device-sdk/opu"
)

func TestCRCAndDocumentVectors(t *testing.T) {
	if got := crc16([]byte("123456789")); got != 0x29b1 {
		t.Fatalf("CRC=%04x", got)
	}
	vectors := []struct {
		cmd     byte
		seq     uint16
		payload []byte
		hex     string
	}{
		{cmdPing, 1, []byte("OPU"), "A5 5A 01 01 01 00 03 00 4F 50 55 42 D7"},
		{cmdPing | 0x80, 1, []byte{0, 'O', 'P', 'U'}, "A5 5A 01 81 01 00 04 00 00 4F 50 55 EC AE"},
		{cmdInfo, 2, nil, "A5 5A 01 02 02 00 00 00 5B E2"},
		{cmdEnableAxis, 3, []byte{0}, "A5 5A 01 20 03 00 01 00 00 45 A5"},
		{cmdMoveSteps, 4, []byte{0, 100, 0, 0, 0, 0, 0, 0, 0}, "A5 5A 01 22 04 00 09 00 00 64 00 00 00 00 00 00 00 8D 76"},
		{cmdMoveSteps | 0x80, 4, []byte{0}, "A5 5A 01 A2 04 00 01 00 00 F1 9D"},
		{cmdStop, 5, []byte{0xff}, "A5 5A 01 25 05 00 01 00 FF 31 35"},
		{cmdEmergencyStop, 6, nil, "A5 5A 01 26 06 00 00 00 18 A9"},
	}
	for _, v := range vectors {
		got, err := encodeFrame(v.cmd, v.seq, v.payload)
		if err != nil {
			t.Fatal(err)
		}
		want := mustHex(v.hex)
		if !bytes.Equal(got, want) {
			t.Errorf("cmd %02x: got %x, want %x", v.cmd, got, want)
		}
	}
}

func mustHex(s string) []byte {
	var out []byte
	for _, part := range strings.Fields(s) {
		var b byte
		for _, c := range part {
			b <<= 4
			if c >= '0' && c <= '9' {
				b += byte(c - '0')
			} else {
				b += byte(c - 'A' + 10)
			}
		}
		out = append(out, b)
	}
	return out
}

func TestParserSplitMergedAndCorrupt(t *testing.T) {
	good, _ := encodeFrame(0x81, 1, []byte{0, 'O', 'P', 'U'})
	other, _ := encodeFrame(0x82, 2, []byte{0})
	bad := append([]byte(nil), good...)
	bad[len(bad)-1] ^= 1
	var p frameParser
	if got := p.feed(append([]byte{0, 0xa5}, good[:4]...)); len(got) != 0 {
		t.Fatal(got)
	}
	got := p.feed(append(append(append([]byte(nil), good[4:]...), bad...), other...))
	if len(got) != 2 || got[0].cmd != 0x81 || got[1].cmd != 0x82 || !bytes.Equal(got[0].payload, []byte{0, 'O', 'P', 'U'}) {
		t.Fatalf("parsed responses: %+v", got)
	}
}

type fakePort struct {
	writes [][]byte
	reads  [][]byte
	answer func([]byte) []byte
}

func (p *fakePort) SetReadTimeout(time.Duration) error { return nil }
func (p *fakePort) Close() error                       { return nil }
func (p *fakePort) Write(b []byte) (int, error) {
	p.writes = append(p.writes, append([]byte(nil), b...))
	if p.answer != nil {
		if reply := p.answer(b); reply != nil {
			p.reads = append(p.reads, reply)
		}
	}
	return len(b), nil
}
func (p *fakePort) Read(b []byte) (int, error) {
	if len(p.reads) == 0 {
		time.Sleep(time.Millisecond)
		return 0, nil
	}
	n := copy(b, p.reads[0])
	p.reads[0] = p.reads[0][n:]
	if len(p.reads[0]) == 0 {
		p.reads = p.reads[1:]
	}
	return n, nil
}

func respond(request []byte, result byte, data []byte) []byte {
	seq := binary.LittleEndian.Uint16(request[4:6])
	frame, _ := encodeFrame(request[3]|0x80, seq, append([]byte{result}, data...))
	return frame
}

func TestClientCommandsAndErrors(t *testing.T) {
	p := &fakePort{}
	p.answer = func(req []byte) []byte { return respond(req, 0, nil) }
	c, err := New(p, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	commands := []struct {
		call    func() error
		cmd     byte
		payload []byte
	}{
		{func() error { return c.EnableAxis(ctx, opu.Azimuth) }, cmdEnableAxis, []byte{0}},
		{func() error { return c.DisableAxis(ctx, opu.Elevation) }, cmdDisableAxis, []byte{1}},
		{func() error { return c.MoveSteps(ctx, opu.Azimuth, -100) }, cmdMoveSteps, []byte{0, 0x9c, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{func() error { return c.MoveRelativeAngle(ctx, opu.Elevation, 12500) }, cmdMoveRelativeAngle, []byte{1, 0xd4, 0x30, 0, 0}},
		{func() error { return c.MoveAbsoluteAngle(ctx, opu.Polarization, -1000) }, cmdMoveAbsoluteAngle, []byte{2, 0x18, 0xfc, 0xff, 0xff}},
		{func() error { return c.Stop(ctx, opu.AllAxes) }, cmdStop, []byte{0xff}},
		{func() error { return c.EmergencyStop(ctx) }, cmdEmergencyStop, nil},
		{func() error { return c.ClearEmergencyStop(ctx) }, cmdClearEmergencyStop, nil},
		{func() error { return c.ZeroEncoder(ctx, opu.Azimuth) }, cmdZeroEncoder, []byte{0}},
		{func() error { return c.ZeroCommandPosition(ctx, opu.Azimuth) }, cmdZeroCommandPosition, []byte{0}},
	}
	for _, item := range commands {
		if err := item.call(); err != nil {
			t.Fatalf("cmd %02x: %v", item.cmd, err)
		}
		last := p.writes[len(p.writes)-1]
		if last[3] != item.cmd || !bytes.Equal(last[8:len(last)-2], item.payload) {
			t.Fatalf("cmd %02x: frame %x", item.cmd, last)
		}
	}
	count := len(p.writes)
	if err := c.MoveSteps(ctx, opu.Azimuth, int64(^uint32(0))+1); err == nil {
		t.Fatal("accepted excess steps")
	}
	if err := c.MoveRelativeAngle(ctx, opu.Azimuth, 36000001); err == nil {
		t.Fatal("accepted excess angle")
	}
	if err := c.Stop(ctx, 3); err == nil {
		t.Fatal("accepted invalid axis")
	}
	if len(p.writes) != count {
		t.Fatal("invalid command sent to device")
	}
	p.answer = func(req []byte) []byte { return respond(req, byte(ResultBusy), nil) }
	err = c.EnableAxis(ctx, opu.Azimuth)
	var result *ResultError
	if !errors.As(err, &result) || result.Code != ResultBusy {
		t.Fatalf("expected BUSY, got %v", err)
	}
}

func TestConfigInfoAndStatus(t *testing.T) {
	config := opu.AxisConfig{Axis: opu.Elevation, MotorFullStepsPerRev: 200, Microsteps: 8,
		GearNumerator: 60, GearDenominator: 1, EncoderCountsPerRev: 8000,
		MaxStepRateHz: 2000, AccelerationStepsS2: 1000, StartStepRateHz: 100, InvertEncoder: true}
	configBytes, err := marshalConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseConfig(configBytes)
	if err != nil || !reflect.DeepEqual(got, config) {
		t.Fatalf("config: %+v, %v", got, err)
	}
	statusBytes := make([]byte, 43)
	statusBytes[0], statusBytes[1], statusBytes[2] = 1, 1, 5
	binary.LittleEndian.PutUint32(statusBytes[3:7], 400)
	binary.LittleEndian.PutUint64(statusBytes[11:19], uint64(100))
	binary.LittleEndian.PutUint64(statusBytes[19:27], uint64(200))
	binary.LittleEndian.PutUint32(statusBytes[27:31], uint32(9000))
	status, err := parseStatus(statusBytes)
	if err != nil || status.Axis != opu.Elevation || !status.Moving || !status.Enabled || !status.IndexSeen || status.EncoderAngleMdeg != 9000 {
		t.Fatalf("status: %+v, %v", status, err)
	}
	infoBytes := []byte{1, 0, 1, 3, 0x20, 0x4e, 0, 0, 0x88, 0x13, 0, 0, 0x0f, 0, 0, 0}
	p := &fakePort{}
	p.answer = func(req []byte) []byte {
		switch req[3] {
		case cmdInfo:
			return respond(req, 0, infoBytes)
		case cmdGetAxisConfig:
			return respond(req, 0, configBytes)
		case cmdGetStatus:
			return respond(req, 0, statusBytes)
		default:
			return respond(req, 0, nil)
		}
	}
	c, err := New(p, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	info, err := c.Info(context.Background())
	if err != nil || info.MotionTickHz != 20000 || info.MaxStepRateHz != 5000 || info.Capabilities != 15 {
		t.Fatalf("info: %+v, %v", info, err)
	}
	got, err = c.AxisConfig(context.Background(), opu.Elevation)
	if err != nil || !reflect.DeepEqual(got, config) {
		t.Fatalf("read config: %+v, %v", got, err)
	}
	status, err = c.Status(context.Background(), opu.Elevation)
	if err != nil || status.RemainingSteps != 400 {
		t.Fatalf("read status: %+v, %v", status, err)
	}
}

func TestTimeoutDoesNotRetryMotion(t *testing.T) {
	p := &fakePort{}
	c, err := New(p, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	err = c.MoveSteps(context.Background(), opu.Azimuth, 100)
	if err == nil || !strings.Contains(err.Error(), "execution is unknown") {
		t.Fatalf("timeout: %v", err)
	}
	if len(p.writes) != 1 {
		t.Fatalf("motion was sent %d times", len(p.writes))
	}
}

func TestClientMatchesSequenceAndRejectsInvalidAck(t *testing.T) {
	p := &fakePort{}
	p.answer = func(req []byte) []byte {
		seq := binary.LittleEndian.Uint16(req[4:6])
		stale, _ := encodeFrame(req[3]|0x80, seq-1, []byte{0})
		return append(stale, respond(req, 0, nil)...)
	}
	c, err := New(p, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnableAxis(context.Background(), opu.Azimuth); err != nil {
		t.Fatal(err)
	}
	p.answer = func(req []byte) []byte { return respond(req, 0, []byte{1}) }
	if err := c.EnableAxis(context.Background(), opu.Azimuth); !errors.Is(err, ErrProtocol) {
		t.Fatalf("expected invalid ACK error, got %v", err)
	}
}
