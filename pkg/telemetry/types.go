package telemetry

// Sensor schema constants (single source of truth).
// Battery voltage in volts, 6-bit quantized.
const (
	BatteryMin  = 10.0
	BatteryMax  = 15.0
	BatteryBits = uint8(6)
)

// Altitude in meters, 14-bit quantized, delta-encodable.
const (
	AltitudeMin  = 0.0
	AltitudeMax  = 5000.0
	AltitudeBits = uint8(14)
)

// IMU acceleration per axis in m/s^2, 10-bit quantized, delta-encodable.
const (
	IMUMin  = -15.0
	IMUMax  = 15.0
	IMUBits = uint8(10)
)

// Packet framing bit widths.
const (
	VersionBits   = uint8(2)
	SequenceBits  = uint8(12)
	FlagsBits     = uint8(4)
	AltitudeMode  = uint8(1) // 1 bit: 0=delta, 1=keyframe
	DeltaBits     = uint8(4) // delta encoding width
	CRCBits       = uint8(8)
)

// Flag bit definitions (4 bits total).
const (
	FlagIMUValid   = uint8(0x01) // bit 0: IMU data is valid
	FlagBatteryOk  = uint8(0x02) // bit 1: battery voltage in nominal range
	FlagCommsReady = uint8(0x04) // bit 2: comms channel ready
	FlagError      = uint8(0x08) // bit 3: error or exception state
)

// Channel describes a sensor in the schema.
type Channel struct {
	Name    string
	Min     float64
	Max     float64
	Bits    uint8
	Deltable bool // true if delta encoding is used for this channel
}

// Schema is the complete sensor configuration set.
var Schema = map[string]Channel{
	"battery": {
		Name:    "Battery",
		Min:     BatteryMin,
		Max:     BatteryMax,
		Bits:    BatteryBits,
		Deltable: false,
	},
	"altitude": {
		Name:    "Altitude",
		Min:     AltitudeMin,
		Max:     AltitudeMax,
		Bits:    AltitudeBits,
		Deltable: true,
	},
	"imu_x": {
		Name:    "IMU_X",
		Min:     IMUMin,
		Max:     IMUMax,
		Bits:    IMUBits,
		Deltable: true,
	},
	"imu_y": {
		Name:    "IMU_Y",
		Min:     IMUMin,
		Max:     IMUMax,
		Bits:    IMUBits,
		Deltable: true,
	},
	"imu_z": {
		Name:    "IMU_Z",
		Min:     IMUMin,
		Max:     IMUMax,
		Bits:    IMUBits,
		Deltable: true,
	},
}
