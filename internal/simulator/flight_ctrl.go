package simulator

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Controller is a virtual flight controller that generates realistic sensor data.
type Controller struct {
	altState   AltitudeState
	battState  BatteryState
	imuState   IMUState
	rng        *rand.Rand
	sampleChan chan SensorSample
	ticker     *time.Ticker
	startTime  time.Time
	seqNum     uint16
}

// NewController creates a new virtual flight controller with deterministic or random seed.
// Set seed to 0 for non-deterministic behavior; otherwise it seeds the RNG.
func NewController(seed int64) *Controller {
	var rng *rand.Rand
	if seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	} else {
		rng = rand.New(rand.NewSource(seed))
	}

	return &Controller{
		altState: AltitudeState{
			BaseAltitude: 2500.0,
			Amplitude:    1200.0,
			Frequency:    0.05, // slow oscillation
			Phase:        0.0,
		},
		battState: BatteryState{
			InitialVoltage: 14.0,
			DischargeRate:  0.001, // slow discharge
			LastSpikeMag:   0.0,
		},
		imuState: IMUState{
			X:           0.0,
			Y:           0.0,
			Z:           0.0,
			NoiseStdDev: 0.5,
		},
		rng:        rng,
		sampleChan: make(chan SensorSample, 1),
		startTime:  time.Now(),
	}
}

// Start begins the 50 ms simulation loop in a goroutine.
func (c *Controller) Start(ctx context.Context) {
	c.ticker = time.NewTicker(50 * time.Millisecond)
	go c.run(ctx)
}

// Samples returns the channel on which samples are emitted.
func (c *Controller) Samples() <-chan SensorSample {
	return c.sampleChan
}

func (c *Controller) run(ctx context.Context) {
	defer c.ticker.Stop()
	defer close(c.sampleChan)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.ticker.C:
			now := time.Now()
			elapsed := now.Sub(c.startTime).Seconds()
			sample := c.generateSample(elapsed, now)
			select {
			case c.sampleChan <- sample:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *Controller) generateSample(elapsed float64, ts time.Time) SensorSample {
	// Altitude: A(t) = A_0 + a*sin(2*pi*f*t) + Gaussian noise
	phase := 2 * math.Pi * c.altState.Frequency * elapsed
	c.altState.Phase = phase
	altNoise := c.rng.NormFloat64() * 10.0 // ~10m noise stddev
	altitude := c.altState.BaseAltitude + c.altState.Amplitude*math.Sin(phase) + altNoise

	// Battery: B(t) = B_0 - k*t + sparse spikes (random motor bursts)
	battery := c.battState.InitialVoltage - c.battState.DischargeRate*elapsed
	// 5% chance of a 0.3V spike downward per sample
	if c.rng.Float64() < 0.05 {
		c.battState.LastSpikeMag = 0.3
		battery -= c.battState.LastSpikeMag
	}

	// IMU: random walk on each axis with Gaussian steps
	imuNoise := [3]float64{
		c.rng.NormFloat64() * c.imuState.NoiseStdDev,
		c.rng.NormFloat64() * c.imuState.NoiseStdDev,
		c.rng.NormFloat64() * c.imuState.NoiseStdDev,
	}
	c.imuState.X += imuNoise[0]
	c.imuState.Y += imuNoise[1]
	c.imuState.Z += imuNoise[2]

	// Clamp IMU to [-15, 15] m/s^2 range
	if c.imuState.X < -15 {
		c.imuState.X = -15
	}
	if c.imuState.X > 15 {
		c.imuState.X = 15
	}
	if c.imuState.Y < -15 {
		c.imuState.Y = -15
	}
	if c.imuState.Y > 15 {
		c.imuState.Y = 15
	}
	if c.imuState.Z < -15 {
		c.imuState.Z = -15
	}
	if c.imuState.Z > 15 {
		c.imuState.Z = 15
	}

	// Flags
	flags := uint8(0x04) // assume comms ready
	if battery > 12.5 && battery < 15.0 {
		flags |= 0x02 // battery OK
	}

	sample := SensorSample{
		Timestamp:   ts,
		Battery:     battery,
		Altitude:    altitude,
		IMU:         [3]float64{c.imuState.X, c.imuState.Y, c.imuState.Z},
		Flags:       flags,
		SequenceNum: c.seqNum,
	}
	c.seqNum++
	return sample
}
