// Package opu defines common contracts and the catalogue of supported
// antenna positioning units (ОПУ).
package opu

import (
	"context"
	"time"
)

type Model string

const STM32V1 Model = "stm32-opu-3d-v1"

type Descriptor struct {
	Model           Model
	Name            string
	ProtocolVersion uint8
	AxisCount       uint8
}

// Supported returns a fresh snapshot of the ОПУ models implemented by the SDK.
func Supported() []Descriptor {
	return []Descriptor{{
		Model: STM32V1, Name: "OPU 3D / STM32F407 USB CDC",
		ProtocolVersion: 1, AxisCount: 3,
	}}
}

type Axis uint8

const (
	Azimuth      Axis = 0
	Elevation    Axis = 1
	Polarization Axis = 2
	AllAxes      Axis = 0xff // Valid only for Stop.
)

type AxisConfig struct {
	Axis                 Axis
	MotorFullStepsPerRev uint16
	Microsteps           uint16
	GearNumerator        uint32
	GearDenominator      uint32
	EncoderCountsPerRev  uint32
	MaxStepRateHz        uint32
	AccelerationStepsS2  uint32
	StartStepRateHz      uint32
	InvertMotor          bool
	InvertEncoder        bool
}

type AxisStatus struct {
	Axis                   Axis
	Moving                 bool
	Enabled                bool
	EmergencyStopped       bool
	IndexSeen              bool
	IndexLevel             bool
	RemainingSteps         uint32
	CurrentStepRateHz      uint32
	CommandedPositionSteps int64
	EncoderCount           int64
	EncoderAngleMdeg       int32 // math.MinInt32 means the encoder scale is not configured.
	IndexCount             int64
	IndexEvents            uint32
}

type Info struct {
	FirmwareMajor   uint8
	FirmwareMinor   uint8
	ProtocolVersion uint8
	AxisCount       uint8
	MotionTickHz    uint32
	MaxStepRateHz   uint32
	Capabilities    uint32
}

const (
	CapabilityStepMotion uint32 = 1 << iota
	CapabilityAngleMotion
	CapabilityEncoderIndex
	CapabilityEmergencyStop
	CapabilityElevationBrake
	CapabilityElevationLimits
	CapabilityElevationReference
)

type BrakeTiming struct {
	PowerWaitMs uint16
	ReleaseMs   uint16
	SettleMs    uint16
	ApplyMs     uint16
}

type ElevationSafetyState uint8

const (
	ElevationLocked ElevationSafetyState = iota
	ElevationPowerWait
	ElevationReleaseWait
	ElevationMoving
	ElevationSettle
	ElevationApplyWait
	ElevationBrakeTest
)

type ElevationFault uint8

const (
	ElevationFaultNone ElevationFault = iota
	ElevationFaultEmergencyStop
	ElevationFaultCoilTimeout
	ElevationFaultForeground
	ElevationFaultLink
	ElevationFaultLimitSwitch
	ElevationFaultPositionLimit
)

type ElevationSafety struct {
	State             ElevationSafetyState
	Fault             ElevationFault
	Referenced        bool
	CoilEnabled       bool
	DriverEnabled     bool
	EmergencyStopped  bool
	HardLimitsPresent bool
	DisablePending    bool
	LowerLimitActive  bool
	UpperLimitActive  bool
	MinAngleMdeg      int32
	MaxAngleMdeg      int32
	CoilOnMs          uint32
	CooldownMs        uint32
	MaxCoilOnMs       uint32
	BrakeTiming       BrakeTiming
	MinSteps          int64
	MaxSteps          int64
}

// Device exposes protocol commands. Move methods acknowledge acceptance only;
// callers must poll Status and check the physical position themselves.
type Device interface {
	Ping(context.Context, []byte) ([]byte, error)
	Info(context.Context) (Info, error)
	SetAxisConfig(context.Context, AxisConfig) error
	AxisConfig(context.Context, Axis) (AxisConfig, error)
	EnableAxis(context.Context, Axis) error
	DisableAxis(context.Context, Axis) error
	MoveSteps(context.Context, Axis, int64) error
	MoveRelativeAngle(context.Context, Axis, int32) error
	MoveAbsoluteAngle(context.Context, Axis, int32) error
	Stop(context.Context, Axis) error
	EmergencyStop(context.Context) error
	ClearEmergencyStop(context.Context) error
	Status(context.Context, Axis) (AxisStatus, error)
	ZeroEncoder(context.Context, Axis) error
	ZeroCommandPosition(context.Context, Axis) error
	ElevationSafety(context.Context) (ElevationSafety, error)
	SetBrakeTiming(context.Context, BrakeTiming) error
	TestBrake(context.Context, time.Duration) error
	ReferenceElevation(context.Context, int32) error
	Close() error
}
