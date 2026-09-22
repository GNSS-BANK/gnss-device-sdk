package stm32

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/GNSS-BANK/gnss-device-sdk/opu"
)

const (
	version    = 1
	maxPayload = 128
	maxFrame   = maxPayload + 10
)

const (
	cmdPing                = 0x01
	cmdInfo                = 0x02
	cmdSetAxisConfig       = 0x10
	cmdGetAxisConfig       = 0x11
	cmdEnableAxis          = 0x20
	cmdDisableAxis         = 0x21
	cmdMoveSteps           = 0x22
	cmdMoveRelativeAngle   = 0x23
	cmdMoveAbsoluteAngle   = 0x24
	cmdStop                = 0x25
	cmdEmergencyStop       = 0x26
	cmdClearEmergencyStop  = 0x27
	cmdGetStatus           = 0x30
	cmdZeroEncoder         = 0x31
	cmdZeroCommandPosition = 0x32
)

type Result uint8

const (
	ResultOK            Result = 0
	ResultInvalidAxis   Result = 1
	ResultInvalidArg    Result = 2
	ResultBusy          Result = 3
	ResultDisabled      Result = 4
	ResultEStop         Result = 5
	ResultNotConfigured Result = 6
	ResultRXOverflow    Result = 7
	ResultBadCRC        Result = 8
	ResultBadHeader     Result = 9
	ResultUnsupported   Result = 10
)

type ResultError struct{ Code Result }

func (e *ResultError) Error() string {
	names := [...]string{"OK", "INVALID_AXIS", "INVALID_ARGUMENT", "BUSY", "DISABLED", "ESTOP", "NOT_CONFIGURED", "RX_OVERFLOW", "BAD_CRC", "BAD_HEADER", "UNSUPPORTED"}
	if int(e.Code) < len(names) {
		return "ОПУ: " + names[e.Code]
	}
	return fmt.Sprintf("ОПУ: unknown result %d", e.Code)
}

var ErrProtocol = errors.New("ОПУ: invalid response frame")

func crc16(data []byte) uint16 {
	crc := uint16(0xffff)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func encodeFrame(cmd uint8, seq uint16, payload []byte) ([]byte, error) {
	if len(payload) > maxPayload {
		return nil, fmt.Errorf("ОПУ: payload exceeds %d bytes", maxPayload)
	}
	frame := make([]byte, len(payload)+10)
	frame[0], frame[1], frame[2], frame[3] = 0xa5, 0x5a, version, cmd
	binary.LittleEndian.PutUint16(frame[4:6], seq)
	binary.LittleEndian.PutUint16(frame[6:8], uint16(len(payload)))
	copy(frame[8:], payload)
	binary.LittleEndian.PutUint16(frame[len(frame)-2:], crc16(frame[2:len(frame)-2]))
	return frame, nil
}

type response struct {
	cmd     uint8
	seq     uint16
	payload []byte
}

// frameParser consumes arbitrary USB read boundaries and discards malformed
// frames one byte at a time to recover the next SOF.
type frameParser struct{ buffer []byte }

func (p *frameParser) feed(data []byte) []response {
	p.buffer = append(p.buffer, data...)
	var frames []response
	for {
		for len(p.buffer) > 0 && (p.buffer[0] != 0xa5 || len(p.buffer) > 1 && p.buffer[1] != 0x5a) {
			p.buffer = p.buffer[1:]
		}
		if len(p.buffer) < 8 {
			break
		}
		n := int(binary.LittleEndian.Uint16(p.buffer[6:8]))
		if p.buffer[2] != version || n > maxPayload {
			p.buffer = p.buffer[1:]
			continue
		}
		if len(p.buffer) < n+10 {
			break
		}
		end := n + 10
		if crc16(p.buffer[2:end-2]) != binary.LittleEndian.Uint16(p.buffer[end-2:end]) {
			p.buffer = p.buffer[1:]
			continue
		}
		frames = append(frames, response{
			cmd: p.buffer[3], seq: binary.LittleEndian.Uint16(p.buffer[4:6]),
			payload: append([]byte(nil), p.buffer[8:end-2]...),
		})
		p.buffer = p.buffer[end:]
	}
	if len(p.buffer) > maxFrame {
		p.buffer = p.buffer[len(p.buffer)-maxFrame:]
	}
	return frames
}

func validAxis(axis opu.Axis) error {
	if axis > opu.Polarization {
		return fmt.Errorf("ОПУ: invalid axis %d", axis)
	}
	return nil
}

func marshalConfig(c opu.AxisConfig) ([]byte, error) {
	if err := validAxis(c.Axis); err != nil {
		return nil, err
	}
	if c.MotorFullStepsPerRev < 1 || c.MotorFullStepsPerRev > 1000 || c.Microsteps < 1 || c.Microsteps > 256 ||
		c.GearNumerator < 1 || c.GearNumerator > 10000 || c.GearDenominator < 1 || c.GearDenominator > 10000 ||
		c.MaxStepRateHz < 1 || c.MaxStepRateHz > 5000 || c.AccelerationStepsS2 < 1 || c.AccelerationStepsS2 > 100000 ||
		c.StartStepRateHz < 1 || c.StartStepRateHz > c.MaxStepRateHz {
		return nil, errors.New("ОПУ: axis configuration is out of range")
	}
	b := make([]byte, 30)
	b[0] = byte(c.Axis)
	binary.LittleEndian.PutUint16(b[1:3], c.MotorFullStepsPerRev)
	binary.LittleEndian.PutUint16(b[3:5], c.Microsteps)
	binary.LittleEndian.PutUint32(b[5:9], c.GearNumerator)
	binary.LittleEndian.PutUint32(b[9:13], c.GearDenominator)
	binary.LittleEndian.PutUint32(b[13:17], c.EncoderCountsPerRev)
	binary.LittleEndian.PutUint32(b[17:21], c.MaxStepRateHz)
	binary.LittleEndian.PutUint32(b[21:25], c.AccelerationStepsS2)
	binary.LittleEndian.PutUint32(b[25:29], c.StartStepRateHz)
	if c.InvertMotor {
		b[29] |= 1
	}
	if c.InvertEncoder {
		b[29] |= 2
	}
	return b, nil
}

func parseConfig(b []byte) (opu.AxisConfig, error) {
	if len(b) != 30 {
		return opu.AxisConfig{}, ErrProtocol
	}
	c := opu.AxisConfig{
		Axis: opu.Axis(b[0]), MotorFullStepsPerRev: binary.LittleEndian.Uint16(b[1:3]),
		Microsteps: binary.LittleEndian.Uint16(b[3:5]), GearNumerator: binary.LittleEndian.Uint32(b[5:9]),
		GearDenominator: binary.LittleEndian.Uint32(b[9:13]), EncoderCountsPerRev: binary.LittleEndian.Uint32(b[13:17]),
		MaxStepRateHz: binary.LittleEndian.Uint32(b[17:21]), AccelerationStepsS2: binary.LittleEndian.Uint32(b[21:25]),
		StartStepRateHz: binary.LittleEndian.Uint32(b[25:29]), InvertMotor: b[29]&1 != 0, InvertEncoder: b[29]&2 != 0,
	}
	if b[29]&^byte(3) != 0 {
		return opu.AxisConfig{}, ErrProtocol
	}
	if _, err := marshalConfig(c); err != nil {
		return opu.AxisConfig{}, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return c, nil
}

func parseInfo(b []byte) (opu.Info, error) {
	if len(b) != 16 {
		return opu.Info{}, ErrProtocol
	}
	return opu.Info{
		FirmwareMajor: b[0], FirmwareMinor: b[1], ProtocolVersion: b[2], AxisCount: b[3],
		MotionTickHz: binary.LittleEndian.Uint32(b[4:8]), MaxStepRateHz: binary.LittleEndian.Uint32(b[8:12]),
		Capabilities: binary.LittleEndian.Uint32(b[12:16]),
	}, nil
}

func parseStatus(b []byte) (opu.AxisStatus, error) {
	if len(b) != 43 || b[1] > 1 || b[2]&^byte(0x0f) != 0 || validAxis(opu.Axis(b[0])) != nil {
		return opu.AxisStatus{}, ErrProtocol
	}
	return opu.AxisStatus{
		Axis: opu.Axis(b[0]), Moving: b[1] == 1, Enabled: b[2]&1 != 0,
		EmergencyStopped: b[2]&2 != 0, IndexSeen: b[2]&4 != 0, IndexLevel: b[2]&8 != 0,
		RemainingSteps: binary.LittleEndian.Uint32(b[3:7]), CurrentStepRateHz: binary.LittleEndian.Uint32(b[7:11]),
		CommandedPositionSteps: int64(binary.LittleEndian.Uint64(b[11:19])),
		EncoderCount:           int64(binary.LittleEndian.Uint64(b[19:27])),
		EncoderAngleMdeg:       int32(binary.LittleEndian.Uint32(b[27:31])),
		IndexCount:             int64(binary.LittleEndian.Uint64(b[31:39])), IndexEvents: binary.LittleEndian.Uint32(b[39:43]),
	}, nil
}
