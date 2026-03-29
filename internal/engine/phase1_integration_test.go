package engine

import (
	"math"
	"testing"

	"prune/pkg/telemetry"
)

func TestPhase1QuantizeAndPackIntegration(t *testing.T) {
	// Simulated raw values that a future TUI pipeline would display as raw vs reconstructed.
	rawBattery := 11.87
	rawAltitude := 3478.42

	bq, err := Quantize(rawBattery, telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)
	if err != nil {
		t.Fatalf("battery quantize failed: %v", err)
	}
	aq, err := Quantize(rawAltitude, telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)
	if err != nil {
		t.Fatalf("altitude quantize failed: %v", err)
	}

	buf := NewBitBuffer()
	if err := buf.WriteBits(uint64(bq), telemetry.BatteryBits); err != nil {
		t.Fatalf("pack battery failed: %v", err)
	}
	if err := buf.WriteBits(uint64(aq), telemetry.AltitudeBits); err != nil {
		t.Fatalf("pack altitude failed: %v", err)
	}

	buf.RewindRead()
	bq2, err := buf.ReadBits(telemetry.BatteryBits)
	if err != nil {
		t.Fatalf("unpack battery failed: %v", err)
	}
	aq2, err := buf.ReadBits(telemetry.AltitudeBits)
	if err != nil {
		t.Fatalf("unpack altitude failed: %v", err)
	}

	if uint32(bq2) != bq {
		t.Fatalf("battery code mismatch: got %d want %d", bq2, bq)
	}
	if uint32(aq2) != aq {
		t.Fatalf("altitude code mismatch: got %d want %d", aq2, aq)
	}

	reconBattery := Dequantize(uint32(bq2), telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)
	reconAltitude := Dequantize(uint32(aq2), telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)

	bStep := QuantizationStep(telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)
	aStep := QuantizationStep(telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)

	if math.Abs(rawBattery-reconBattery) > bStep/2+1e-12 {
		t.Fatalf("battery reconstruction error too high: raw=%.12f recon=%.12f", rawBattery, reconBattery)
	}
	if math.Abs(rawAltitude-reconAltitude) > aStep/2+1e-12 {
		t.Fatalf("altitude reconstruction error too high: raw=%.12f recon=%.12f", rawAltitude, reconAltitude)
	}

	// The bitstream string is needed for the future dashboard visualizer panel.
	if len(buf.BitString()) != int(telemetry.BatteryBits+telemetry.AltitudeBits) {
		t.Fatalf("unexpected bitstring length: got %d", len(buf.BitString()))
	}
}
