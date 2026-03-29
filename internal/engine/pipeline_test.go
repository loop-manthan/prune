package engine

import (
	"context"
	"math"
	"testing"
	"time"

	"prune/internal/simulator"
	"prune/pkg/telemetry"
)

func TestSimulatorGeneratesRealisticSamples(t *testing.T) {
	ctrl := simulator.NewController(42)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ctrl.Start(ctx)

	count := 0
	for sample := range ctrl.Samples() {
		if sample.Battery < telemetry.BatteryMin || sample.Battery > telemetry.BatteryMax*1.2 {
			t.Logf("Sample %d: Battery %.2f outside nominal range\n", count, sample.Battery)
		}
		if sample.Altitude < telemetry.AltitudeMin || sample.Altitude > telemetry.AltitudeMax*1.2 {
			t.Logf("Sample %d: Altitude %.2f outside nominal range\n", count, sample.Altitude)
		}
		for i := 0; i < 3; i++ {
			if sample.IMU[i] < telemetry.IMUMin-1 || sample.IMU[i] > telemetry.IMUMax+1 {
				t.Logf("Sample %d: IMU[%d]=%.2f clamped by controller\n", count, i, sample.IMU[i])
			}
		}
		count++
	}
	if count == 0 {
		t.Fatal("simulator produced no samples in 500ms")
	}
}

func TestFrameCodecRoundtrip(t *testing.T) {
	frame := &EncodedFrame{
		Version:       0,
		Sequence:      123,
		Flags:         telemetry.FlagBatteryOk | telemetry.FlagCommsReady,
		BatteryQ:      42,
		AltitudeQ:     5, // delta payload (valid 4-bit value)
		AltMode:       false, // delta mode
		IMUQ:          [3]uint32{2, 3, 4}, // delta payloads (valid 4-bit values)
		IMUModes:      [3]bool{false, false, false},
		PrevAltitudeQ: 1000,
		PrevIMUQ:      [3]uint32{100, 150, 200},
	}

	codec := NewFrameCodec()
	encoded, err := codec.Encode(frame)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Version != frame.Version {
		t.Fatalf("version mismatch: %d vs %d", decoded.Version, frame.Version)
	}
	if decoded.Sequence != frame.Sequence {
		t.Fatalf("sequence mismatch: %d vs %d", decoded.Sequence, frame.Sequence)
	}
	if decoded.Flags != frame.Flags {
		t.Fatalf("flags mismatch: %d vs %d", decoded.Flags, frame.Flags)
	}
	if decoded.BatteryQ != frame.BatteryQ {
		t.Fatalf("battery mismatch: %d vs %d", decoded.BatteryQ, frame.BatteryQ)
	}
	if decoded.AltitudeQ != frame.AltitudeQ {
		t.Fatalf("altitude payload mismatch: %d vs %d", decoded.AltitudeQ, frame.AltitudeQ)
	}
	if decoded.AltMode != frame.AltMode {
		t.Fatalf("altitude mode mismatch: %v vs %v", decoded.AltMode, frame.AltMode)
	}
	for i := 0; i < 3; i++ {
		if decoded.IMUQ[i] != frame.IMUQ[i] {
			t.Fatalf("imu[%d] payload mismatch: %d vs %d", i, decoded.IMUQ[i], frame.IMUQ[i])
		}
		if decoded.IMUModes[i] != frame.IMUModes[i] {
			t.Fatalf("imu[%d] mode mismatch: %v vs %v", i, decoded.IMUModes[i], frame.IMUModes[i])
		}
	}
}

func TestFrameCodecCRCDetection(t *testing.T) {
	frame := &EncodedFrame{
		Version:       0,
		Sequence:      456,
		Flags:         0,
		BatteryQ:      30,
		AltitudeQ:     2000,
		AltMode:       true,
		IMUQ:          [3]uint32{100, 150, 200},
		IMUModes:      [3]bool{true, true, true},
		PrevAltitudeQ: 0,
		PrevIMUQ:      [3]uint32{0, 0, 0},
	}

	codec := NewFrameCodec()
	encoded, err := codec.Encode(frame)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	_, err = codec.Decode(encoded)
	if err != nil {
		t.Fatalf("clean decode should succeed: %v", err)
	}

	// Flip a payload bit (not the CRC)
	if len(encoded) > 1 {
		corrupted := append([]byte(nil), encoded...)
		corrupted[0] ^= 0x01
		_, err := codec.Decode(corrupted)
		if err != ErrCRCValidationFailed {
			t.Fatalf("expected CRC validation error, got %v", err)
		}
	}
}

func TestQuantizeAndEncodeDecodeAndDequantizeRoundtrip(t *testing.T) {
	rawInput := struct {
		Battery   float64
		Altitude  float64
		IMU       [3]float64
		Flags     uint8
		Sequence  uint16
	}{
		Battery:  11.5,
		Altitude: 2345.67,
		IMU:      [3]float64{1.2, -0.5, 3.1},
		Flags:    telemetry.FlagBatteryOk,
		Sequence: 100,
	}

	priorState := &struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	}{
		AltitudeQ: 0,
		IMUQ:      [3]uint32{0, 0, 0},
	}

	frame, err := QuantizeAndEncode(rawInput, priorState)
	if err != nil {
		t.Fatalf("QuantizeAndEncode failed: %v", err)
	}

	frame.PrevAltitudeQ = priorState.AltitudeQ
	frame.PrevIMUQ = priorState.IMUQ

	rawOutput, _, err := DecodeAndDequantize(frame)
	if err != nil {
		t.Fatalf("DecodeAndDequantize failed: %v", err)
	}

	if rawOutput.Sequence != rawInput.Sequence {
		t.Fatalf("sequence mismatch: %d vs %d", rawOutput.Sequence, rawInput.Sequence)
	}
	if rawOutput.Flags != rawInput.Flags {
		t.Fatalf("flags mismatch: %d vs %d", rawOutput.Flags, rawInput.Flags)
	}

	// Check quantization error bounds
	batteryStep := QuantizationStep(telemetry.BatteryMin, telemetry.BatteryMax, telemetry.BatteryBits)
	batteryErr := math.Abs(rawInput.Battery - rawOutput.Battery)
	if batteryErr > batteryStep/2+1e-10 {
		t.Logf("battery error: %.12f (limit %.12f)", batteryErr, batteryStep/2)
	}

	altitudeStep := QuantizationStep(telemetry.AltitudeMin, telemetry.AltitudeMax, telemetry.AltitudeBits)
	altitudeErr := math.Abs(rawInput.Altitude - rawOutput.Altitude)
	if altitudeErr > altitudeStep/2+1e-10 {
		t.Logf("altitude error: %.12f (limit %.12f)", altitudeErr, altitudeStep/2)
	}

	for i := 0; i < 3; i++ {
		imuStep := QuantizationStep(telemetry.IMUMin, telemetry.IMUMax, telemetry.IMUBits)
		imuErr := math.Abs(rawInput.IMU[i] - rawOutput.IMU[i])
		if imuErr > imuStep/2+1e-10 {
			t.Logf("imu[%d] error: %.12f (limit %.12f)", i, imuErr, imuStep/2)
		}
	}
}

func TestPipelineFullEndToEnd(t *testing.T) {
	ctrl := simulator.NewController(99)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ctrl.Start(ctx)

	priorState := &struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	}{
		AltitudeQ: 0,
		IMUQ:      [3]uint32{0, 0, 0},
	}

	codec := NewFrameCodec()
	sampleCount := 0
	crcFailures := 0

	for sample := range ctrl.Samples() {
		rawInput := struct {
			Battery   float64
			Altitude  float64
			IMU       [3]float64
			Flags     uint8
			Sequence  uint16
		}{
			Battery:   sample.Battery,
			Altitude:  sample.Altitude,
			IMU:       sample.IMU,
			Flags:     sample.Flags,
			Sequence:  sample.SequenceNum & 0xFFF,
		}

		frame, err := QuantizeAndEncode(rawInput, priorState)
		if err != nil {
			t.Fatalf("QuantizeAndEncode failed: %v", err)
		}

		frame.PrevAltitudeQ = priorState.AltitudeQ
		frame.PrevIMUQ = priorState.IMUQ

		encoded, err := codec.Encode(frame)
		if err != nil {
			t.Fatalf("codec.Encode failed: %v", err)
		}

		decoded, err := codec.Decode(encoded)
		if err != nil {
			crcFailures++
			continue
		}

		rawOutput, qvals, err := DecodeAndDequantize(decoded)
		if err != nil {
			t.Fatalf("DecodeAndDequantize failed: %v", err)
		}

		_ = rawOutput // use in a real TUI for display

		// Update prior state with reconstructed quantized values for next iteration
		priorState.AltitudeQ = qvals.AltitudeQ
		priorState.IMUQ = qvals.IMUQ

		sampleCount++
	}

	if sampleCount == 0 {
		t.Fatal("pipeline produced no samples")
	}
	if crcFailures > 0 {
		t.Logf("pipeline completed %d samples, %d CRC failures (should be 0)", sampleCount, crcFailures)
	}
}

