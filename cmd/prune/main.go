package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"prune/internal/engine"
	"prune/internal/simulator"
	"prune/internal/tui"
)

func main() {
	model := tui.NewModel(120, 40)

	// Initialize simulator and pipeline
	ctrl := simulator.NewController(42) // deterministic seed for reproducibility
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	codec := engine.NewFrameCodec()
	priorState := &struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	}{
		AltitudeQ: 0,
		IMUQ:      [3]uint32{0, 0, 0},
	}

	var stateMu sync.Mutex

	ctrl.Start(ctx)

	// Start sample processor in background
	go processSamples(model, ctrl, codec, priorState, &stateMu)

	// Start Bubble Tea app
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	cancel()
}

func processSamples(
	model *tui.TUIModel,
	ctrl *simulator.Controller,
	codec *engine.FrameCodec,
	priorState *struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	},
	stateMu *sync.Mutex,
) {
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

		// Measure encode latency
		t0 := time.Now()
		frame, err := engine.QuantizeAndEncode(rawInput, priorState)
		if err != nil {
			continue
		}

		frame.PrevAltitudeQ = priorState.AltitudeQ
		frame.PrevIMUQ = priorState.IMUQ

		encoded, err := codec.Encode(frame)
		t1 := time.Now()

		if err != nil {
			continue
		}

		// Measure decode latency
		decoded, err := codec.Decode(encoded)
		if err != nil {
			continue
		}

		rawOutput, qvals, err := engine.DecodeAndDequantize(decoded)
		if err != nil {
			continue
		}
		t2 := time.Now()

		// Compute quantization errors
		errors := [5]float64{
			math.Abs(rawInput.Battery - rawOutput.Battery),
			math.Abs(rawInput.Altitude - rawOutput.Altitude),
			math.Abs(rawInput.IMU[0] - rawOutput.IMU[0]),
			math.Abs(rawInput.IMU[1] - rawOutput.IMU[1]),
			math.Abs(rawInput.IMU[2] - rawOutput.IMU[2]),
		}

		encodeLatency := t1.Sub(t0).Microseconds()
		decodeLatency := t2.Sub(t1).Microseconds()

		// Estimate JSON size as ~80 bytes per sample
		jsonSize := int64(80)
		packedSize := int64(len(encoded))

		// Extract bitstring for visualization
		bitstring := ""
		if len(encoded) > 1 {
			payload := encoded[:len(encoded)-1]
			buf := engine.NewBitBufferFromBytes(payload)
			bitstring = buf.BitString()
			if len(bitstring) > 64 {
				bitstring = bitstring[:64]
			}
		}

		// Update prior state
		stateMu.Lock()
		priorState.AltitudeQ = qvals.AltitudeQ
		priorState.IMUQ = qvals.IMUQ
		stateMu.Unlock()

		// Prepare raw and quantized sensor arrays for display
		rawSensors := [5]float64{
			rawInput.Battery,
			rawInput.Altitude,
			rawInput.IMU[0],
			rawInput.IMU[1],
			rawInput.IMU[2],
		}

		// Extract quantized values (dequantized from packed data for display)
		quantSensors := [5]float64{
			rawOutput.Battery,
			rawOutput.Altitude,
			rawOutput.IMU[0],
			rawOutput.IMU[1],
			rawOutput.IMU[2],
		}

		// Record metrics directly to model
		model.Metrics.RecordSample(
			encodeLatency,
			decodeLatency,
			jsonSize,
			packedSize,
			errors,
			bitstring,
			err == nil,
			rawSensors,
			quantSensors,
		)
	}
}

