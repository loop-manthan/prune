package tui

import (
	"math"
	"time"
)

// MetricsCollector tracks telemetry and compression performance.
type MetricsCollector struct {
	// Sample counts
	TotalSamples     int64
	CRCFailures      int64
	CRCSuccesses     int64

	// Compression metrics
	TotalJSONBytes   int64
	TotalPackedBytes int64

	// Quantization error tracking (per channel)
	// [battery, altitude, imu_x, imu_y, imu_z]
	QuantizationErrors [5]struct {
		SumSquaredError float64
		Count           int64
	}

	// Latest sensor values (real-time data)
	LatestBattery float64
	LatestAltitude float64
	LatestIMU     [3]float64 // [x, y, z]

	// Latest quantized values (for display)
	LatestBatteryQ float64
	LatestAltitudeQ float64
	LatestIMUQ     [3]float64

	// Latency tracking (in microseconds)
	LastEncodeLatency     int64 // us
	LastDecodeLatency     int64 // us
	LastRoundtripLatency  int64 // us
	AvgEncodeLatency      float64
	AvgDecodeLatency      float64
	AvgRoundtripLatency   float64

	// Bitstream visualization
	RecentBitstrings []string // ring buffer of last N bitstrings
	MaxBitstrings    int
}

// NewMetricsCollector creates a metrics tracker.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		RecentBitstrings: make([]string, 0, 20),
		MaxBitstrings:    20,
	}
}

// RecordSample updates metrics after processing one sensor sample.
func (m *MetricsCollector) RecordSample(
	encodeLatency, decodeLatency int64, // in microseconds
	jsonBytes, packedBytes int64,
	quantErrs [5]float64, // per-channel absolute errors
	bitstring string,
	crcOK bool,
	rawSensors [5]float64, // [battery, altitude, imu_x, imu_y, imu_z]
	quantSensors [5]float64, // quantized versions
) {
	m.TotalSamples++
	m.TotalJSONBytes += jsonBytes
	m.TotalPackedBytes += packedBytes

	// Store latest sensor values
	m.LatestBattery = rawSensors[0]
	m.LatestAltitude = rawSensors[1]
	m.LatestIMU[0] = rawSensors[2]
	m.LatestIMU[1] = rawSensors[3]
	m.LatestIMU[2] = rawSensors[4]

	m.LatestBatteryQ = quantSensors[0]
	m.LatestAltitudeQ = quantSensors[1]
	m.LatestIMUQ[0] = quantSensors[2]
	m.LatestIMUQ[1] = quantSensors[3]
	m.LatestIMUQ[2] = quantSensors[4]

	if crcOK {
		m.CRCSuccesses++
	} else {
		m.CRCFailures++
	}

	// Update encode/decode latencies
	m.LastEncodeLatency = encodeLatency
	m.LastDecodeLatency = decodeLatency
	m.LastRoundtripLatency = encodeLatency + decodeLatency

	// Rolling average (exponential smoothing)
	alpha := 0.1 // weight for new sample
	m.AvgEncodeLatency = alpha*float64(encodeLatency) + (1-alpha)*m.AvgEncodeLatency
	m.AvgDecodeLatency = alpha*float64(decodeLatency) + (1-alpha)*m.AvgDecodeLatency
	m.AvgRoundtripLatency = alpha*float64(m.LastRoundtripLatency) + (1-alpha)*m.AvgRoundtripLatency

	// Track quantization errors for RMSE
	for i := 0; i < 5; i++ {
		m.QuantizationErrors[i].SumSquaredError += quantErrs[i] * quantErrs[i]
		m.QuantizationErrors[i].Count++
	}

	// Ring buffer for bitstring visualization
	if len(m.RecentBitstrings) >= m.MaxBitstrings {
		m.RecentBitstrings = m.RecentBitstrings[1:]
	}
	m.RecentBitstrings = append(m.RecentBitstrings, bitstring)
}

// CompressionRatio returns JSON bytes / packed bytes.
func (m *MetricsCollector) CompressionRatio() float64 {
	if m.TotalPackedBytes == 0 {
		return 1.0
	}
	return float64(m.TotalJSONBytes) / float64(m.TotalPackedBytes)
}

// SavingsPercent returns (1 - packed/json) * 100.
func (m *MetricsCollector) SavingsPercent() float64 {
	if m.TotalJSONBytes == 0 {
		return 0.0
	}
	return (1.0 - float64(m.TotalPackedBytes)/float64(m.TotalJSONBytes)) * 100.0
}

// CRCSuccessRate returns success count / total count.
func (m *MetricsCollector) CRCSuccessRate() float64 {
	total := m.CRCSuccesses + m.CRCFailures
	if total == 0 {
		return 1.0
	}
	return float64(m.CRCSuccesses) / float64(total)
}

// RMSE returns per-channel root mean square error.
func (m *MetricsCollector) RMSE() [5]float64 {
	var rmse [5]float64
	for i := 0; i < 5; i++ {
		if m.QuantizationErrors[i].Count > 0 {
			rmse[i] = math.Sqrt(m.QuantizationErrors[i].SumSquaredError / float64(m.QuantizationErrors[i].Count))
		}
	}
	return rmse
}

// ChannelNames returns the channel labels for display.
func ChannelNames() [5]string {
	return [5]string{"Battery", "Altitude", "IMU-X", "IMU-Y", "IMU-Z"}
}

// ViewMode represents which screen to display.
type ViewMode int

const (
	ViewCompression ViewMode = iota
	ViewErrors
	ViewLatency
	ViewCRC
	ViewBitstream
	ViewStats
)

// TUIModel is the Bubble Tea state for the dashboard.
type TUIModel struct {
	Metrics     *MetricsCollector
	Width       int
	Height      int
	Tick        int64
	StartTime   time.Time
	RunningTime time.Duration
	CurrentView ViewMode
}

// NewModel creates the initial TUI model.
func NewModel(width, height int) *TUIModel {
	return &TUIModel{
		Metrics:     NewMetricsCollector(),
		Width:       width,
		Height:      height,
		StartTime:   time.Now(),
		CurrentView: ViewCompression,
	}
}

// NextView cycles to the next view mode.
func (m *TUIModel) NextView() {
	m.CurrentView = (m.CurrentView + 1) % 6
}

// PrevView cycles to the previous view mode.
func (m *TUIModel) PrevView() {
	m.CurrentView = (m.CurrentView - 1 + 6) % 6
}
