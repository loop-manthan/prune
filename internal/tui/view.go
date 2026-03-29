package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorCyan    = lipgloss.Color("51")
	colorGreen   = lipgloss.Color("46")
	colorYellow  = lipgloss.Color("226")
	colorBlue    = lipgloss.Color("33")
	colorWhite   = lipgloss.Color("255")
	colorBlack   = lipgloss.Color("16")
	colorMagenta = lipgloss.Color("205")
	colorRed     = lipgloss.Color("196")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorBlue).
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
)

func (m *TUIModel) Render() string {
	if m.Width < 90 || m.Height < 24 {
		return "Terminal too small (need 90x24)"
	}

	header := styleHeader.Render(fmt.Sprintf("Prune | source=%s | samples=%d | runtime=%s", m.SourceName, m.Metrics.TotalSamples, m.RunningTime.Truncate(10e6)))

	bodyHeight := m.Height - 3
	leftW := m.Width / 3
	rightW := m.Width - leftW - 1
	scienceH := bodyHeight / 2
	vizH := bodyHeight - scienceH - 1

	left := stylePanel.Width(leftW - 2).Height(bodyHeight - 2).Render(
		stylePanelTitle.Render("Part 1: Comparisons") + "\n" + renderComparisons(m),
	)

	science := stylePanel.Width(rightW - 2).Height(scienceH - 2).Render(
		stylePanelTitle.Render("Part 2: Current System State") + "\n" + renderSystemState(m),
	)

	viz := stylePanel.Width(rightW - 2).Height(vizH - 2).Render(
		stylePanelTitle.Render("Part 3: Live Data Visualization") + "\n" + renderAltitudeBatteryGraph(rightW-6, vizH-6, m.Metrics),
	)

	right := lipgloss.JoinVertical(lipgloss.Left, science, viz)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	help := styleHelp.Render("q: quit | single screen layout: comparisons + plain-English science + live visualization")
	return strings.Join([]string{header, body, help}, "\n")
}

func renderComparisons(m *TUIModel) string {
	rmse := m.Metrics.RMSE()
	crc := m.Metrics.CRCSuccessRate() * 100
	ratio := m.Metrics.CompressionRatio()
	savings := m.Metrics.SavingsPercent()

	lines := []string{
		fmt.Sprintf("Compression ratio: %s", styleGood.Render(fmt.Sprintf("%.2fx", ratio))),
		fmt.Sprintf("Bandwidth savings: %s", styleGood.Render(fmt.Sprintf("%.1f%%", savings))),
		fmt.Sprintf("CRC success: %s", styleGood.Render(fmt.Sprintf("%.2f%%", crc))),
		"",
		fmt.Sprintf("Raw battery:        %6.2f V", m.Metrics.LatestBattery),
		fmt.Sprintf("Reconstructed batt: %6.2f V", m.Metrics.LatestBatteryQ),
		fmt.Sprintf("Battery error:      %6.4f", rmse[0]),
		"",
		fmt.Sprintf("Raw altitude:       %8.2f m", m.Metrics.LatestAltitude),
		fmt.Sprintf("Reconstructed alt:  %8.2f m", m.Metrics.LatestAltitudeQ),
		fmt.Sprintf("Altitude error:     %8.4f", rmse[1]),
		"",
		fmt.Sprintf("Encode latency: %4d us", m.Metrics.LastEncodeLatency),
		fmt.Sprintf("Decode latency: %4d us", m.Metrics.LastDecodeLatency),
		fmt.Sprintf("Roundtrip avg:  %.1f us", m.Metrics.AvgRoundtripLatency),
		"",
		fmt.Sprintf("JSON bytes:   %d", m.Metrics.TotalJSONBytes),
		fmt.Sprintf("Packed bytes: %d", m.Metrics.TotalPackedBytes),
	}
	return strings.Join(lines, "\n")
}

func renderSystemState(m *TUIModel) string {
	rmse := m.Metrics.RMSE()
	sampleRate := 0.0
	if m.Metrics.TotalSamples > 0 && m.RunningTime.Seconds() > 0 {
		sampleRate = float64(m.Metrics.TotalSamples) / m.RunningTime.Seconds()
	}

	lines := []string{
		fmt.Sprintf("Real-time sample rate: %.1f Hz", sampleRate),
		fmt.Sprintf("Altitude trending: %s (%.1f m)", trendIndicator(m.Metrics.AltitudeHistory), m.Metrics.LatestAltitude),
		fmt.Sprintf("Battery health: %s (%.2f V)", batteryStatus(m.Metrics.LatestBattery), m.Metrics.LatestBattery),
		fmt.Sprintf("IMU X stability: %s (%.2f m/s²)", imuIndicator(rmse[2]), m.Metrics.LatestIMU[0]),
		"",
		fmt.Sprintf("Compression is reducing raw data size by %s%%.", styleGood.Render(fmt.Sprintf("%.0f", m.Metrics.SavingsPercent()))),
		fmt.Sprintf("Altitude delta encoding saves bits by diff-encoding signal changes."),
		fmt.Sprintf("Battery sent absolute because voltage is non-monotonic and critical."),
		fmt.Sprintf("IMU axis tracked independently; motion is 3D and uncorrelated."),
		"",
		fmt.Sprintf("Quantization RMSE - Alt: %.3f, Batt: %.3f, IMU: [%.3f, %.3f, %.3f]",
			rmse[1], rmse[0], rmse[2], rmse[3], rmse[4]),
		fmt.Sprintf("Encoded frames average %.1f bits each, decoded in %.1f microseconds.",
			estimateFrameBits(m.Metrics), m.Metrics.AvgRoundtripLatency),
	}
	return strings.Join(lines, "\n")
}

func trendIndicator(history []float64) string {
	if len(history) < 2 {
		return "→"
	}
	recent := history[len(history)-1]
	older := history[len(history)/2]
	if recent > older+50 {
		return styleGood.Render("↑ rising")
	} else if recent < older-50 {
		return styleBad.Render("↓ falling")
	}
	return "→ stable"
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

func renderAltitudeBatteryGraph(width, height int, m *MetricsCollector) string {
	if width < 20 || height < 6 {
		return "Graph area too small"
	}

	var b strings.Builder
	b.WriteString("Altitude (meters):\n")

	if len(m.AltitudeHistory) > 0 {
		altGraph := renderLineGraph(m.AltitudeHistory, width, 3, 0, 5000)
		b.WriteString(altGraph)
	} else {
		b.WriteString(strings.Repeat(" ", width) + "\n")
	}

	b.WriteString("\nBattery (volts):\n")
	if len(m.BatteryHistory) > 0 {
		battGraph := renderLineGraph(m.BatteryHistory, width, 2, 10, 15)
		b.WriteString(battGraph)
	} else {
		b.WriteString(strings.Repeat(" ", width) + "\n")
	}

	return b.String()
}

func renderLineGraph(history []float64, width, height int, minVal, maxVal float64) string {
	if len(history) == 0 || width < 5 {
		return ""
	}

	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
		}
	}

	rangeVal := maxVal - minVal
	if rangeVal <= 0 {
		rangeVal = 1
	}

	step := len(history) / width
	if step < 1 {
		step = 1
	}

	for x := 0; x < width && x*step < len(history); x++ {
		idx := x * step
		if idx >= len(history) {
			break
		}
		raw := history[idx]
		normalized := (raw - minVal) / rangeVal
		normalized = math.Max(0, math.Min(1, normalized))
		y := height - 1 - int(normalized*float64(height-1))
		if y >= 0 && y < height {
			grid[y][x] = '*'
		}
	}

	var result strings.Builder
	for y := 0; y < height; y++ {
		result.WriteString(string(grid[y]))
		result.WriteString("\n")
	}
	return result.String()
}
