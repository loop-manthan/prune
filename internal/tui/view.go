package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorCyan    = lipgloss.Color("51")
	colorGreen   = lipgloss.Color("46")
	colorYellow  = lipgloss.Color("226")
	colorNavy    = lipgloss.Color("24")
	colorWhite   = lipgloss.Color("255")
	colorGray    = lipgloss.Color("245")
	colorBlack   = lipgloss.Color("16")
	colorMagenta = lipgloss.Color("205")
	colorRed     = lipgloss.Color("196")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorNavy).
			Padding(0, 1)

	stylePanel = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1)

	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlack).
			Background(colorYellow).
			Padding(0, 1)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorMagenta).
			Padding(0, 1)

	styleGood   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleBad    = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleMetric = lipgloss.NewStyle().Foreground(colorCyan)
	styleMuted  = lipgloss.NewStyle().Foreground(colorGray)
	styleAccent = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
)

func (m *TUIModel) Render() string {
	if m.Width < 90 || m.Height < 24 {
		return "Terminal too small (need 90x24)"
	}

	sampleRate := 0.0
	if m.Metrics.TotalSamples > 0 && m.RunningTime.Seconds() > 0 {
		sampleRate = float64(m.Metrics.TotalSamples) / m.RunningTime.Seconds()
	}

	header := styleHeader.Render(fmt.Sprintf(
		"PRUNE  source=%s  samples=%d  runtime=%s  rate=%.1fHz",
		m.SourceName,
		m.Metrics.TotalSamples,
		m.RunningTime.Truncate(10*time.Millisecond),
		sampleRate,
	))

	bodyHeight := m.Height - 3
	leftW := m.Width * 38 / 100
	rightW := m.Width - leftW - 1
	topH := bodyHeight * 45 / 100
	bottomH := bodyHeight - topH - 1

	left := stylePanel.Width(leftW - 2).Height(bodyHeight - 2).Render(
		stylePanelTitle.Render("Part 1: Comparisons")+"\n"+renderComparisons(m),
	)

	top := stylePanel.Width(rightW - 2).Height(topH - 2).Render(
		stylePanelTitle.Render("Operational Signals")+"\n"+renderOperationsPanel(m, rightW-6),
	)

	bottom := stylePanel.Width(rightW - 2).Height(bottomH - 2).Render(
		stylePanelTitle.Render("Part 3: Real-time Data + Compressed Bits")+"\n"+renderVisualizationPanel(m, rightW-6, bottomH-6),
	)

	right := lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	help := styleHelp.Render("q: quit | left: full comparisons | top-right: operational signals | bottom-right: live data + compressed bits")
	return strings.Join([]string{header, body, help}, "\n")
}

func renderComparisons(m *TUIModel) string {
	crc := m.Metrics.CRCSuccessRate() * 100
	ratio := m.Metrics.CompressionRatio()
	savings := m.Metrics.SavingsPercent()
	avgBits := estimateFrameBits(m.Metrics)

	avgJSON := 0.0
	avgPacked := 0.0
	if m.Metrics.TotalSamples > 0 {
		avgJSON = float64(m.Metrics.TotalJSONBytes) / float64(m.Metrics.TotalSamples)
		avgPacked = float64(m.Metrics.TotalPackedBytes) / float64(m.Metrics.TotalSamples)
	}

	lines := []string{
		styleAccent.Render("Compression Snapshot"),
		fmt.Sprintf("Bandwidth savings:  %s", styleGood.Render(fmt.Sprintf("%.1f%%", savings))),
		fmt.Sprintf("Compression ratio:  %s", styleGood.Render(fmt.Sprintf("%.2fx", ratio))),
		fmt.Sprintf("CRC success rate:   %s", styleGood.Render(fmt.Sprintf("%.2f%%", crc))),
		fmt.Sprintf("Avg frame payload:  %s", styleMetric.Render(fmt.Sprintf("%.1f bits", avgBits))),
		"",
		styleAccent.Render("Payload Footprint"),
		fmt.Sprintf("Total JSON bytes:   %d", m.Metrics.TotalJSONBytes),
		fmt.Sprintf("Total packed bytes: %d", m.Metrics.TotalPackedBytes),
		fmt.Sprintf("Per sample:         %.1fB JSON  ->  %.1fB packed", avgJSON, avgPacked),
		fmt.Sprintf("Delta per sample:   %.1fB saved", avgJSON-avgPacked),
		"",
		styleAccent.Render("How It Works"),
		"1) Quantization: each sensor is mapped into a fixed numeric range.",
		"   This turns floating-point values into compact integers with bounded error.",
		"2) Delta mode: for altitude and IMU axes, we usually send change from last sample.",
		"   Most real signals move gradually, so small deltas cost fewer bits than full values.",
		"3) Adaptive framing: when a change is too large, encoder switches to keyframe mode.",
		"   This keeps decoding stable during sudden motion while preserving compression overall.",
		"4) Bit packing + CRC: fields are packed at bit-level, then protected with CRC-8.",
		"   Result: lower bandwidth usage with integrity checks before decoded data is trusted.",
	}
	return strings.Join(lines, "\n")
}

func renderOperationsPanel(m *TUIModel, width int) string {
	rmse := m.Metrics.RMSE()
	sampleRate := 0.0
	if m.Metrics.TotalSamples > 0 && m.RunningTime.Seconds() > 0 {
		sampleRate = float64(m.Metrics.TotalSamples) / m.RunningTime.Seconds()
	}

	altSpark := renderSparkline(m.Metrics.AltitudeHistory, 0, 5000, width-18)
	battSpark := renderSparkline(m.Metrics.BatteryHistory, 10, 15, width-18)
	densitySpark := renderSparkline(deriveBitDensityHistory(m.Metrics.RecentBitstrings), 0, 1, width-18)

	lines := []string{
		fmt.Sprintf("rate %.1fHz   trend %s   battery %s", sampleRate, trendIndicator(m.Metrics.AltitudeHistory), batteryStatus(m.Metrics.LatestBattery)),
		fmt.Sprintf("avg frame %.1fbits   codec %.1fus", estimateFrameBits(m.Metrics), m.Metrics.AvgRoundtripLatency),
		"",
		styleMuted.Render("altitude trend") + "  " + altSpark,
		styleMuted.Render("battery trend ") + "  " + battSpark,
		styleMuted.Render("bit density  ") + "  " + densitySpark,
		"",
		fmt.Sprintf("rmse batt %.4f   alt %.3f   imu [%.3f %.3f %.3f]", rmse[0], rmse[1], rmse[2], rmse[3], rmse[4]),
		styleMuted.Render("status: altitude is delta-coded; battery is absolute; IMU is per-axis adaptive"),
	}
	return strings.Join(lines, "\n")
}

func renderVisualizationPanel(m *TUIModel, width, height int) string {
	if width < 24 || height < 10 {
		return "Panel too small"
	}
	latestBits := "(waiting for first frame...)"
	if n := len(m.Metrics.RecentBitstrings); n > 0 {
		latestBits = m.Metrics.RecentBitstrings[n-1]
	}

	wrappedBits := wrapText(latestBits, width-2)
	rmse := m.Metrics.RMSE()

	lines := []string{
		styleAccent.Render("Real-time Sensor Data"),
		fmt.Sprintf("Battery   raw=%6.2f V   reconstructed=%6.2f V   rmse=%.4f", m.Metrics.LatestBattery, m.Metrics.LatestBatteryQ, rmse[0]),
		fmt.Sprintf("Altitude  raw=%8.2f m   reconstructed=%8.2f m   rmse=%.4f", m.Metrics.LatestAltitude, m.Metrics.LatestAltitudeQ, rmse[1]),
		fmt.Sprintf("IMU X/Y/Z raw=[%6.2f %6.2f %6.2f]", m.Metrics.LatestIMU[0], m.Metrics.LatestIMU[1], m.Metrics.LatestIMU[2]),
		fmt.Sprintf("IMU X/Y/Z rec=[%6.2f %6.2f %6.2f]", m.Metrics.LatestIMUQ[0], m.Metrics.LatestIMUQ[1], m.Metrics.LatestIMUQ[2]),
		"",
		styleAccent.Render("Compressed Payload Bits (latest frame)"),
		wrappedBits,
		"",
		fmt.Sprintf("Payload density: %.1f%% ones", latestBitDensity(latestBits)*100),
	}

	return strings.Join(lines, "\n")
}

func renderSparkline(history []float64, min, max float64, width int) string {
	if width <= 0 {
		return ""
	}
	if len(history) == 0 {
		return styleMuted.Render(strings.Repeat("·", width))
	}

	levels := []rune("▁▂▃▄▅▆▇█")
	vals := plotHistoryToColumns(history, width)
	out := make([]rune, len(vals))
	for i, v := range vals {
		norm := clamp01((v - min) / (max - min))
		idx := int(math.Round(norm * float64(len(levels)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(levels) {
			idx = len(levels) - 1
		}
		out[i] = levels[idx]
	}
	return styleMetric.Render(string(out))
}

func plotHistoryToColumns(history []float64, width int) []float64 {
	if len(history) == 0 || width <= 0 {
		return nil
	}
	out := make([]float64, 0, width)
	for x := 0; x < width; x++ {
		idx := int(math.Round(float64(x) * float64(len(history)-1) / float64(width-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(history) {
			idx = len(history) - 1
		}
		out = append(out, history[idx])
	}
	return out
}


func wrapText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}

	var out []string
	for i := 0; i < len(s); i += width {
		end := i + width
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return strings.Join(out, "\n")
}

func latestBitDensity(bitstring string) float64 {
	if len(bitstring) == 0 || bitstring == "(waiting for first frame...)" {
		return 0
	}
	ones := 0
	for i := 0; i < len(bitstring); i++ {
		if bitstring[i] == '1' {
			ones++
		}
	}
	return float64(ones) / float64(len(bitstring))
}

func deriveBitDensityHistory(bits []string) []float64 {
	out := make([]float64, 0, len(bits))
	for _, frame := range bits {
		if len(frame) == 0 {
			out = append(out, 0)
			continue
		}
		ones := 0
		for i := 0; i < len(frame); i++ {
			if frame[i] == '1' {
				ones++
			}
		}
		out = append(out, float64(ones)/float64(len(frame)))
	}
	return out
}

func trendIndicator(history []float64) string {
	if len(history) < 2 {
		return styleMuted.Render("steady")
	}
	recent := history[len(history)-1]
	older := history[len(history)/2]
	if recent > older+50 {
		return styleGood.Render("rising")
	} else if recent < older-50 {
		return styleBad.Render("falling")
	}
	return styleMuted.Render("stable")
}

func batteryStatus(v float64) string {
	if v > 13.5 {
		return styleGood.Render("good")
	} else if v > 12 {
		return styleMetric.Render("ok")
	}
	return styleBad.Render("low")
}

func imuIndicator(rmse float64) string {
	if rmse < 0.2 {
		return styleGood.Render("low-noise")
	}
	return styleMetric.Render("nominal")
}

func estimateFrameBits(m *MetricsCollector) float64 {
	if m.TotalSamples == 0 {
		return 0
	}
	totalBits := float64(m.TotalPackedBytes) * 8.0
	return totalBits / float64(m.TotalSamples)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
