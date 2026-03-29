package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Vibrant color scheme
	colorMagenta   = lipgloss.Color("205") // bright magenta
	colorCyan      = lipgloss.Color("51")  // bright cyan
	colorGreen     = lipgloss.Color("46")  // bright green
	colorYellow    = lipgloss.Color("226") // bright yellow
	colorRed       = lipgloss.Color("196") // bright red
	colorBlue      = lipgloss.Color("33")  // bright blue
	colorWhite     = lipgloss.Color("255") // white
	colorBlack     = lipgloss.Color("16")  // black
	colorDarkGray  = lipgloss.Color("235") // dark gray

	// Large bold styles for main metrics
	styleMainValue = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorBlue).
		Padding(2, 4)

	styleMainCompress = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorMagenta).
		Padding(2, 4)

	styleMainError = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorCyan).
		Padding(2, 4)

	styleMainLatency = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorGreen).
		Padding(2, 4)

	styleMainCRC = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorRed).
		Padding(2, 4)

	styleSecondary = lipgloss.NewStyle().
		Foreground(colorYellow)

	styleLabel = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorCyan)

	styleHelpBar = lipgloss.NewStyle().
		Foreground(colorBlack).
		Background(colorYellow).
		Bold(true).
		Padding(0, 1)

	stylePageIndicator = lipgloss.NewStyle().
		Foreground(colorMagenta).
		Bold(true)
)

// RenderCompressionView shows compression ratio full-screen with sensor data.
func RenderCompressionView(m *TUIModel, width int, height int) string {
	ratio := m.Metrics.CompressionRatio()
	savings := m.Metrics.SavingsPercent()
	jsonB := float64(m.Metrics.TotalJSONBytes) / 1024.0
	packedB := float64(m.Metrics.TotalPackedBytes) / 1024.0
	reduction := jsonB - packedB
	samples := m.Metrics.TotalSamples

	// Make percentage the main metric (huge!)
	mainMetric := fmt.Sprintf("%.1f%%", savings)
	mainStr := styleMainCompress.Render(mainMetric)

	// Real-time sensor data with detailed quantization theory
	sensorInfo := fmt.Sprintf(
		"\n%s\n"+
			"► Live Battery: %.2f V  |  Live Altitude: %.1f m  |  Live IMU: [%.2f, %.2f, %.2f] m/s²\n\n"+
			"%s LINEAR QUANTIZATION THEORY:\n"+
			"Converts continuous floating-point values to discrete integer levels using formula:\n"+
			"  q = round[(v - v_min) / (v_max - v_min) × (2^k - 1)], where k = bit depth\n"+
			"Then reconstructs via: v̂ = v_min + q/(2^k - 1) × (v_max - v_min)\n"+
			"This guarantees max error ≤ LSB/2. Battery [10-15V, 6-bit]: 0.0794 V/LSB. Altitude [0-5000m, 14-bit]: 0.305 m/LSB.\n"+
			"IMU [±15m/s², 10-bit]: 0.0293 m/s²/LSB. All reconstructed values provably bounded within range.\n\n"+
			"%s BANDWIDTH REDUCTION:\n"+
			"Ratio: %.2fx | Savings: %.1f%% | Reduction: %.1f KB | Total Samples: %d | JSON: %.1f KB → Packed: %.1f KB\n",
		styleLabel.Render("═══ REAL-TIME SENSOR DATA ═══"),
		m.Metrics.LatestBattery,
		m.Metrics.LatestAltitude,
		m.Metrics.LatestIMU[0],
		m.Metrics.LatestIMU[1],
		m.Metrics.LatestIMU[2],
		styleLabel.Render("═══ QUANTIZATION SCIENCE ═══"),
		styleLabel.Render("═══"),
		ratio,
		savings,
		reduction,
		samples,
		jsonB,
		packedB,
	)

	content := mainStr + sensorInfo

	return content
}

// RenderErrorView shows quantization error (RMSE) full-screen.
func RenderErrorView(m *TUIModel, width int, height int) string {
	rmse := m.Metrics.RMSE()
	names := ChannelNames()

	// Find max RMSE for highlighting
	maxRMSE := 0.0
	maxIdx := 0
	for i := 0; i < 5; i++ {
		if rmse[i] > maxRMSE {
			maxRMSE = rmse[i]
			maxIdx = i
		}
	}

	mainMetric := fmt.Sprintf("%.4f", rmse[maxIdx])
	mainStr := styleMainError.Render(mainMetric)

	var details strings.Builder
	details.WriteString(fmt.Sprintf("\n%s: %s (worst channel RMSE)\n\n", styleLabel.Render("REAL-TIME RECONSTRUCTED VALUES"), styleSecondary.Render(names[maxIdx])))

	// Show reconstructed quantized values (what you get after decompression)
	details.WriteString(fmt.Sprintf("► Battery: %.2f V (from quant) | Altitude: %.1f m | IMU: [%.2f, %.2f, %.2f] m/s²\n\n", 
		m.Metrics.LatestBatteryQ, m.Metrics.LatestAltitudeQ, 
		m.Metrics.LatestIMUQ[0], m.Metrics.LatestIMUQ[1], m.Metrics.LatestIMUQ[2]))

	// Per-channel RMSE with detailed specs
	specs := []string{
		"[6-bit]   10-15V",
		"[14-bit]  0-5000m (delta)",
		"[10-bit]  ±15m/s² (delta)",
		"[10-bit]  ±15m/s² (delta)",
		"[10-bit]  ±15m/s² (delta)",
	}

	for i := 0; i < 5; i++ {
		barLen := 30
		filled := int(float64(barLen) * math.Min(rmse[i]/1.0, 1.0))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)

		color := colorGreen
		if rmse[i] > 0.5 {
			color = colorYellow
		}
		if rmse[i] > 1.0 {
			color = colorRed
		}

		style := lipgloss.NewStyle().Foreground(color).Bold(true)
		details.WriteString(fmt.Sprintf("%s %s [%s] RMSE: %.4f\n", names[i], specs[i], style.Render(bar), rmse[i]))
	}

	details.WriteString(fmt.Sprintf("\n%s DELTA ENCODING THEORY:\n", styleLabel.Render("═══ LOSSLESS TEMPORAL COMPRESSION ═══")))
	details.WriteString("For slowly-changing channels (Altitude, IMU), encode temporal deltas δ = q_t - q_{t-1} instead of absolute values.\n")
	details.WriteString("Deltas fit in 4-bit signed range [-8, 7] quantized units. If overflow occurs, emit full keyframe with 14-bit altitude or 10-bit IMU.\n")
	details.WriteString("This exploits correlation: aircraft altitude changes ~20-50 quant levels per sample; 4-bit delta saves 10 bits per packet vs keyframe.\n")
	details.WriteString(fmt.Sprintf("Sample count: %d  |  Running RMSE is cumulative over all reconstructed samples\n", m.Metrics.TotalSamples))

	content := mainStr + details.String()

	return content
}

// RenderLatencyView shows latency full-screen.
func RenderLatencyView(m *TUIModel, width int, height int) string {
	lastRT := m.Metrics.LastRoundtripLatency
	avgRT := m.Metrics.AvgRoundtripLatency

	mainMetric := fmt.Sprintf("%d μs", lastRT)
	mainStr := styleMainLatency.Render(mainMetric)

	var details strings.Builder

	details.WriteString(fmt.Sprintf("\n%s\n", styleLabel.Render("REAL-TIME PIPELINE LATENCY")))
	details.WriteString(fmt.Sprintf("Rolling Avg Roundtrip: %s μs  |  This Sample: %d μs\n\n", 
		styleSecondary.Render(fmt.Sprintf("%.1f", avgRT)), lastRT))

	details.WriteString(fmt.Sprintf("%s Encode (last): %6d μs  |  Avg: %7.1f μs  [quantize → bit-pack → crc]\n", 
		styleSecondary.Render("►"), m.Metrics.LastEncodeLatency, m.Metrics.AvgEncodeLatency))
	details.WriteString(fmt.Sprintf("%s Decode (last): %6d μs  |  Avg: %7.1f μs  [unpack bits → dequantize → validate]\n\n", 
		styleSecondary.Render("►"), m.Metrics.LastDecodeLatency, m.Metrics.AvgDecodeLatency))

	details.WriteString(fmt.Sprintf("%s BIT-PACKING SCIENCE:\n", styleLabel.Render("═══ NON-BYTE-ALIGNED FIELD PACKING ═══")))
	details.WriteString("Prune packs variable-width fields with zero wasted alignment bits using MSB-first bit ordering.\n")
	details.WriteString("Example frame: Version[2] + Seq[12] + Flags[4] + Battery[6] + Altitude[1+4/14] + IMU[1+4/10]×3 + CRC[8] = 46-78 bits total.\n")
	details.WriteString("Bit-buffer reads/writes at arbitrary offsets (0-7): Write 6-bit value starting at bit offset 18, next write at offset 24 (not aligned to byte).\n")
	details.WriteString("This eliminates padding bytes that plague byte-aligned serialization. Encode/decode both O(n) complexity in bit count.\n")
	details.WriteString(fmt.Sprintf("Current cycle: %d samples | 50 Hz tick = 20 ms between samples | Latency < 1ms = excellent real-time performance\n", m.Metrics.TotalSamples))

	content := mainStr + details.String()

	return content
}

// RenderCRCView shows CRC integrity full-screen.
func RenderCRCView(m *TUIModel, width int, height int) string {
	total := m.Metrics.CRCSuccesses + m.Metrics.CRCFailures
	rate := m.Metrics.CRCSuccessRate() * 100

	bgColor := colorGreen
	if rate < 99.9 {
		bgColor = colorYellow
	}
	if rate < 99.0 {
		bgColor = colorRed
	}

	styleMainCRCDyn := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(bgColor).
		Padding(2, 4)

	mainMetric := fmt.Sprintf("%.2f%%", rate)
	mainStr := styleMainCRCDyn.Render(mainMetric)

	status := "✓ EXCELLENT"
	if rate < 99.9 {
		status = "⚠ WARNING"
	}
	if rate < 99.0 {
		status = "✗ CRITICAL"
	}

	// Calculate failure stats
	failureRate := 0.0
	if total > 0 {
		failureRate = float64(m.Metrics.CRCFailures) / float64(total) * 100.0
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("\n%s: Link Integrity\n", styleLabel.Render("REAL-TIME CORRUPTION DETECTION")))
	details.WriteString(fmt.Sprintf("%s Status: %s  |  Corrupt: %.3f%% of packets\n\n", 
		styleSecondary.Render("►"), status, failureRate))

	details.WriteString(fmt.Sprintf("%s Total Packets: %8d [samples: %d @ 50Hz]\n", 
		styleSecondary.Render("►"), total, m.Metrics.TotalSamples))
	details.WriteString(fmt.Sprintf("%s ✓ Valid/OK:    %8d (%.2f%%)\n", 
		styleSecondary.Render("►"), m.Metrics.CRCSuccesses, rate))
	details.WriteString(fmt.Sprintf("%s ✗ Failed/Bad:  %8d (%.2f%%)\n\n", 
		styleSecondary.Render("✗"), m.Metrics.CRCFailures, failureRate))

	details.WriteString(fmt.Sprintf("%s CYCLIC REDUNDANCY CHECK (CRC-8) ALGORITHM:\n", styleLabel.Render("═══ ERROR DETECTION SCIENCE ═══")))
	details.WriteString("CRC-8 polynomial: x^8 + x^2 + x^1 + x^0 = 0x07 (standard ATM polynomial).\n")
	details.WriteString("Bitwise update: for each data byte b: crc ^= b; then 8 times: if (crc & 0x80) crc = (crc << 1) ^ 0x07 else crc <<= 1.\n")
	details.WriteString("Guarantees: detects ALL single-bit errors, 99.6% of 2-bit errors, most burst errors. Rate = CRC-8/256 = 3.125% overhead.\n")
	details.WriteString("Receiver validates: XOR(crc_computed, crc_received) must equal 0. Corrupted payload → reject before decode.\n")
	details.WriteString(fmt.Sprintf("Network health: %.3f%% failure rate indicates link quality (clean = <0.1%%, noisy = >1%%)\n", failureRate))

	content := mainStr + details.String()

	return content
}

// RenderBitstreamView shows raw bitstream.
func RenderBitstreamView(m *TUIModel, width int, height int) string {
	title := styleLabel.Render("LIVE BITSTREAM PACKETS")

	var content strings.Builder
	content.WriteString("\n" + title + "\n")
	content.WriteString(fmt.Sprintf("%s Real-time packet samples: Version[2] + Seq[12] + Flags[4] + Data + CRC[8] = 46-78 bits\n\n", styleSecondary.Render("►")))

	if len(m.Metrics.RecentBitstrings) == 0 {
		content.WriteString(styleSecondary.Render("(Waiting for samples...)"))
	} else {
		// Show last 12 bitstrings
		start := len(m.Metrics.RecentBitstrings) - 12
		if start < 0 {
			start = 0
		}

		for i := start; i < len(m.Metrics.RecentBitstrings); i++ {
			bs := m.Metrics.RecentBitstrings[i]
			idx := i + 1

			// Color alternating packets
			color := colorCyan
			if idx%2 == 0 {
				color = colorMagenta
			}
			style := lipgloss.NewStyle().Foreground(color).Bold(true)

			// Show length
			bitLen := len(bs)
			byteLen := (bitLen + 7) / 8

			if len(bs) > width-20 {
				bs = bs[:width-23] + "..."
			}
			content.WriteString(fmt.Sprintf("[%2d] %s [%d bits / %dB]\n", idx, style.Render(bs), bitLen, byteLen))
		}
	}

	content.WriteString(fmt.Sprintf("\n%s PACKET SERIALIZATION (MSB-first, non-aligned):\n", styleLabel.Render("═══ BIT-LEVEL FRAMING ═══")))
	content.WriteString("Example: Battery[6b]=52 → bits 110100. Altitude mode[1b]=0 (delta) Δalt[4b]=7 → bits 0111.\n")
	content.WriteString("Packed: 0x...110100-0-0111... with zero padding. No byte alignment → saves bytes vs JSON bloat.\n")
	content.WriteString("Advantage: arbitrary-width fields eliminate padding. Disadvantage: bit-extraction O(1) per field via look-ahead buffer.\n")
	content.WriteString("Result: 80 bytes JSON → ~10 bytes packed (8x compression) with bounded reconstruction error.\n")

	return content.String()
}

// RenderStatsView shows session statistics.
func RenderStatsView(m *TUIModel, width int, height int) string {
	sampleRate := 0.0
	if m.Metrics.TotalSamples > 0 && m.RunningTime.Seconds() > 0 {
		sampleRate = float64(m.Metrics.TotalSamples) / m.RunningTime.Seconds()
	}

	mainMetric := fmt.Sprintf("%d", m.Metrics.TotalSamples)
	mainStr := styleMainValue.Render(mainMetric)

	var details strings.Builder
	details.WriteString(fmt.Sprintf("\n%s\n", styleLabel.Render("SESSION STATISTICS & SENSOR MODEL")))
	details.WriteString(fmt.Sprintf("%s Runtime:     %s  |  Rate: %.1f Hz (target: 50 Hz)\n\n", 
		styleSecondary.Render("►"), m.RunningTime.String(), sampleRate))

	details.WriteString(fmt.Sprintf("%s SIMULATED FLIGHT CONTROLLER DYNAMICS:\n", styleLabel.Render("═══ SYNTHETIC SENSOR DATA ═══")))
	details.WriteString("Altitude(t) = 10 + 800·sin(2π·0.05·t) + ε_alt, where ε ~ N(0, 1) m. Range: [0, 5000] m.\n")
	details.WriteString("  ↳ Sine wave frequency = 0.05 Hz (period 20s). Noise σ=1m. Delta encodes to 4-bit signed range [-8,7].\n")
	details.WriteString("Battery(t) = 14.8 - 0.001·t + ε_batt, where ε ~ U[-0.02, 0.02] V. Range: [10, 15] V.\n")
	details.WriteString("  ↳ Linear discharge 1mV per second (5% over 25 minutes). No delta, 6-bit absolute quantization.\n")
	details.WriteString("IMU_axis(t) = IMU_axis(t-1) + η, where η ~ N(0, 0.5). Range: [±15] m/s². Delta encodes per axis.\n")
	details.WriteString("  ↳ Random walk correlated over ~3-5 samples. 10-bit per axis. Deterministic seed=42 for reproducibility.\n\n")

	details.WriteString(fmt.Sprintf("%s Samples recorded: %d  |  Duration: %.1f seconds  |  Expected: ~%d samples @ 50Hz\n", 
		styleSecondary.Render("►"), m.Metrics.TotalSamples, m.RunningTime.Seconds(), int(m.RunningTime.Seconds()*50)))

	content := mainStr + details.String()

	return content
}

// RenderHelpBar shows keyboard shortcuts.
func RenderHelpBar() string {
	help := " 1-6: View  ← →: Navigate  Q: Quit "
	return styleHelpBar.Render(help)
}

// RenderPageIndicator shows which view is active.
func RenderPageIndicator(m *TUIModel) string {
	viewNames := []string{"Compression", "Errors", "Latency", "CRC", "Bitstream", "Stats"}
	indicator := fmt.Sprintf("[%d/6] %s", int(m.CurrentView)+1, viewNames[m.CurrentView])
	return stylePageIndicator.Render(indicator)
}

// Render assembles the full dashboard based on current view.
func (m *TUIModel) Render() string {
	if m.Width < 60 || m.Height < 20 {
		return "Terminal too small (need 60x20)"
	}

	// Reduce usable height for help bar and title
	usableHeight := m.Height - 3

	var mainView string
	switch m.CurrentView {
	case ViewCompression:
		mainView = RenderCompressionView(m, m.Width, usableHeight)
	case ViewErrors:
		mainView = RenderErrorView(m, m.Width, usableHeight)
	case ViewLatency:
		mainView = RenderLatencyView(m, m.Width, usableHeight)
	case ViewCRC:
		mainView = RenderCRCView(m, m.Width, usableHeight)
	case ViewBitstream:
		mainView = RenderBitstreamView(m, m.Width, usableHeight)
	case ViewStats:
		mainView = RenderStatsView(m, m.Width, usableHeight)
	}

	// Assemble: page indicator, main view, help bar
	dashboard := strings.Join([]string{
		RenderPageIndicator(m),
		mainView,
		RenderHelpBar(),
	}, "\n")

	return dashboard
}
