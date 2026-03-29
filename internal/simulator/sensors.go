package simulator

import "time"

// SensorSample represents one timestamped snapshot of all sensors.
type SensorSample struct {
	Timestamp   time.Time
	Battery     float64 // volts
	Altitude    float64 // meters
	IMU         [3]float64 // [x, y, z] acceleration in m/s^2
	Flags       uint8
	SequenceNum uint16
}

// AltitudeState tracks altitude model state for continuous generation.
type AltitudeState struct {
	BaseAltitude float64 // starting altitude A_0
	Amplitude    float64 // sine amplitude a
	Frequency    float64 // frequency f in Hz
	Phase        float64 // accumulated phase
}

// BatteryState tracks battery discharge and random spikes.
type BatteryState struct {
	InitialVoltage float64 // B_0
	DischargeRate  float64 // k (volts per second)
	LastSpikeMag   float64 // most recent spike magnitude
}

// IMUState tracks 3D random walk state.
type IMUState struct {
	X      float64 // accumulated value axis X
	Y      float64 // accumulated value axis Y
	Z      float64 // accumulated value axis Z
	NoiseStdDev float64 // sigma for random walk
}
