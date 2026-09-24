// Package stm32 implements the OPU 3D STM32F407 USB CDC binary protocol v1.
package stm32

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/GNSS-BANK/gnss-device-sdk/opu"
	"go.bug.st/serial"
)

const (
	defaultTimeout = 2 * time.Second
	readTimeout    = 50 * time.Millisecond
)

var _ opu.Device = (*Client)(nil)

// Port is the byte-stream transport required by the client. The standard
// serial.Port satisfies it; tests and integrations may supply another port.
type Port interface {
	io.ReadWriteCloser
	SetReadTimeout(time.Duration) error
}

type Client struct {
	mu      sync.Mutex
	port    Port
	timeout time.Duration
	seq     uint16
	parser  frameParser
	closed  bool
}

// New takes ownership of port. A short read timeout lets context cancellation
// and the per-request deadline interrupt a response wait.
func New(port Port, timeout time.Duration) (*Client, error) {
	if port == nil {
		return nil, errors.New("ОПУ: nil port")
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if err := port.SetReadTimeout(readTimeout); err != nil {
		return nil, fmt.Errorf("ОПУ: set read timeout: %w", err)
	}
	return &Client{port: port, timeout: timeout}, nil
}

// Open opens a USB CDC COM/tty port at 115200 8N1, then verifies PING and
// GET_INFO. Opening does not enable axes or start motion.
func Open(ctx context.Context, path string) (*Client, error) {
	if path == "" {
		return nil, errors.New("ОПУ: empty port path")
	}
	port, err := serial.Open(path, &serial.Mode{
		BaudRate: 115200, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{},
	})
	if err != nil {
		return nil, fmt.Errorf("ОПУ: open %s: %w", path, err)
	}
	c, err := New(port, defaultTimeout)
	if err != nil {
		port.Close()
		return nil, err
	}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	var echo []byte
	echo, err = c.Ping(ctx, []byte("OPU"))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(echo, []byte("OPU")) {
		err = ErrProtocol
		return nil, err
	}
	_, err = c.Info(ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.port.Close()
}

func (c *Client) request(ctx context.Context, cmd uint8, payload []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("ОПУ: port is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.seq++
	seq := c.seq
	frame, err := encodeFrame(cmd, seq, payload)
	if err != nil {
		return nil, err
	}
	for len(frame) > 0 {
		n, writeErr := c.port.Write(frame)
		if writeErr != nil {
			return nil, fmt.Errorf("ОПУ: write failed; execution is unknown: %w", writeErr)
		}
		if n <= 0 || n > len(frame) {
			return nil, errors.New("ОПУ: incomplete write; execution is unknown")
		}
		frame = frame[n:]
	}
	deadline := time.Now().Add(c.timeout)
	var buf [256]byte
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("ОПУ: response wait interrupted; execution is unknown: %w", err)
		}
		if !time.Now().Before(deadline) {
			c.parser.buffer = nil
			return nil, errors.New("ОПУ: response timeout; execution is unknown")
		}
		n, readErr := c.port.Read(buf[:])
		if readErr != nil {
			return nil, fmt.Errorf("ОПУ: read failed; execution is unknown: %w", readErr)
		}
		for _, reply := range c.parser.feed(buf[:n]) {
			if reply.cmd != cmd|0x80 || reply.seq != seq {
				continue
			}
			if len(reply.payload) < 1 {
				return nil, ErrProtocol
			}
			if reply.payload[0] != 0 {
				if len(reply.payload) != 1 {
					return nil, ErrProtocol
				}
				return nil, &ResultError{Code: Result(reply.payload[0])}
			}
			return reply.payload[1:], nil
		}
	}
}

func (c *Client) ack(ctx context.Context, cmd uint8, payload []byte) error {
	b, err := c.request(ctx, cmd, payload)
	if err != nil {
		return err
	}
	if len(b) != 0 {
		return ErrProtocol
	}
	return nil
}

func (c *Client) Ping(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) > 32 {
		return nil, errors.New("ОПУ: PING exceeds 32 bytes")
	}
	b, err := c.request(ctx, cmdPing, data)
	if err != nil {
		return nil, err
	}
	if len(b) != len(data) || !bytes.Equal(b, data) {
		return nil, ErrProtocol
	}
	return b, nil
}

func (c *Client) Info(ctx context.Context) (opu.Info, error) {
	b, err := c.request(ctx, cmdInfo, nil)
	if err != nil {
		return opu.Info{}, err
	}
	info, err := parseInfo(b)
	if err != nil {
		return opu.Info{}, err
	}
	if info.ProtocolVersion != version || info.AxisCount != 3 {
		return opu.Info{}, fmt.Errorf("ОПУ: incompatible protocol %d or axis count %d", info.ProtocolVersion, info.AxisCount)
	}
	return info, nil
}

func (c *Client) SetAxisConfig(ctx context.Context, config opu.AxisConfig) error {
	b, err := marshalConfig(config)
	if err != nil {
		return err
	}
	return c.ack(ctx, cmdSetAxisConfig, b)
}

func (c *Client) AxisConfig(ctx context.Context, axis opu.Axis) (opu.AxisConfig, error) {
	if err := validAxis(axis); err != nil {
		return opu.AxisConfig{}, err
	}
	b, err := c.request(ctx, cmdGetAxisConfig, []byte{byte(axis)})
	if err != nil {
		return opu.AxisConfig{}, err
	}
	config, err := parseConfig(b)
	if err != nil {
		return opu.AxisConfig{}, err
	}
	if config.Axis != axis {
		return opu.AxisConfig{}, ErrProtocol
	}
	return config, nil
}

func (c *Client) EnableAxis(ctx context.Context, axis opu.Axis) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	return c.ack(ctx, cmdEnableAxis, []byte{byte(axis)})
}

func (c *Client) DisableAxis(ctx context.Context, axis opu.Axis) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	return c.ack(ctx, cmdDisableAxis, []byte{byte(axis)})
}

func (c *Client) MoveSteps(ctx context.Context, axis opu.Axis, steps int64) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	if steps < -int64(^uint32(0)) || steps > int64(^uint32(0)) {
		return errors.New("ОПУ: step count exceeds uint32 magnitude")
	}
	b := make([]byte, 9)
	b[0] = byte(axis)
	binary.LittleEndian.PutUint64(b[1:], uint64(steps))
	return c.ack(ctx, cmdMoveSteps, b)
}

func (c *Client) moveAngle(ctx context.Context, cmd uint8, axis opu.Axis, mdeg int32) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	if mdeg < -36000000 || mdeg > 36000000 {
		return errors.New("ОПУ: angle exceeds ±36000000 mdeg")
	}
	b := make([]byte, 5)
	b[0] = byte(axis)
	binary.LittleEndian.PutUint32(b[1:], uint32(mdeg))
	return c.ack(ctx, cmd, b)
}

func (c *Client) MoveRelativeAngle(ctx context.Context, axis opu.Axis, mdeg int32) error {
	return c.moveAngle(ctx, cmdMoveRelativeAngle, axis, mdeg)
}

func (c *Client) MoveAbsoluteAngle(ctx context.Context, axis opu.Axis, mdeg int32) error {
	return c.moveAngle(ctx, cmdMoveAbsoluteAngle, axis, mdeg)
}

func (c *Client) Stop(ctx context.Context, axis opu.Axis) error {
	if axis != opu.AllAxes {
		if err := validAxis(axis); err != nil {
			return err
		}
	}
	return c.ack(ctx, cmdStop, []byte{byte(axis)})
}

func (c *Client) EmergencyStop(ctx context.Context) error { return c.ack(ctx, cmdEmergencyStop, nil) }
func (c *Client) ClearEmergencyStop(ctx context.Context) error {
	return c.ack(ctx, cmdClearEmergencyStop, nil)
}

func (c *Client) Status(ctx context.Context, axis opu.Axis) (opu.AxisStatus, error) {
	if err := validAxis(axis); err != nil {
		return opu.AxisStatus{}, err
	}
	b, err := c.request(ctx, cmdGetStatus, []byte{byte(axis)})
	if err != nil {
		return opu.AxisStatus{}, err
	}
	status, err := parseStatus(b)
	if err != nil {
		return opu.AxisStatus{}, err
	}
	if status.Axis != axis {
		return opu.AxisStatus{}, ErrProtocol
	}
	return status, nil
}

func (c *Client) ZeroEncoder(ctx context.Context, axis opu.Axis) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	return c.ack(ctx, cmdZeroEncoder, []byte{byte(axis)})
}

func (c *Client) ZeroCommandPosition(ctx context.Context, axis opu.Axis) error {
	if err := validAxis(axis); err != nil {
		return err
	}
	return c.ack(ctx, cmdZeroCommandPosition, []byte{byte(axis)})
}

func (c *Client) ElevationSafety(ctx context.Context) (opu.ElevationSafety, error) {
	b, err := c.request(ctx, cmdGetElevationSafety, nil)
	if err != nil {
		return opu.ElevationSafety{}, err
	}
	return parseElevationSafety(b)
}

func (c *Client) SetBrakeTiming(ctx context.Context, timing opu.BrakeTiming) error {
	b, err := marshalBrakeTiming(timing)
	if err != nil {
		return err
	}
	return c.ack(ctx, cmdSetBrakeTiming, b)
}

func (c *Client) TestBrake(ctx context.Context, duration time.Duration) error {
	if duration < time.Millisecond || duration >= 2500*time.Millisecond || duration%time.Millisecond != 0 {
		return errors.New("ОПУ: brake test duration must be an integer number of milliseconds in [1, 2499]")
	}
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(duration/time.Millisecond))
	return c.ack(ctx, cmdTestBrake, b)
}

func (c *Client) ReferenceElevation(ctx context.Context, angleMdeg int32) error {
	if angleMdeg < -25000 || angleMdeg > 90000 {
		return errors.New("ОПУ: elevation reference angle is outside [-25000, 90000] mdeg")
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], uint32(angleMdeg))
	binary.LittleEndian.PutUint32(b[4:8], elevationReferenceKey)
	return c.ack(ctx, cmdReferenceElevation, b)
}
