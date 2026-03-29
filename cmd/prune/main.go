package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"prune/internal/engine"
	"prune/internal/simulator"
	"prune/internal/source"
	"prune/internal/tui"
)

func main() {
	var sourceName string
	var seed int64
	var apiPoll time.Duration

	flag.StringVar(&sourceName, "source", "simulator", "telemetry source: simulator|opensky")
	flag.Int64Var(&seed, "seed", 42, "seed for simulator source (0 = random)")
	flag.DurationVar(&apiPoll, "api-poll", time.Second, "poll interval for API sources")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	src := buildSource(sourceName, seed, apiPoll)
	if err := src.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "source %q unavailable (%v); falling back to simulator\n", src.Name(), err)
		src = source.NewSimulatorSource(seed)
		if err := src.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to start fallback source: %v\n", err)
			os.Exit(1)
		}
	}

	model := tui.NewModel(120, 40, src.Name())

	codec := engine.NewFrameCodec()
	priorState := &struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	}{
		AltitudeQ: 0,
		IMUQ:      [3]uint32{0, 0, 0},
	}

	// Start sample processor in background
	go processSamples(model, src.Samples(), codec, priorState)

	// Start Bubble Tea app
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	cancel()
}

func buildSource(name string, seed int64, pollEvery time.Duration) source.Source {
	switch name {
	case "opensky":
		return source.NewOpenSkySource(pollEvery)
	default:
		return source.NewSimulatorSource(seed)
	}
}

func processSamples(
	model *tui.TUIModel,
	samples <-chan simulator.SensorSample,
	codec *engine.FrameCodec,
	priorState *struct {
		AltitudeQ uint32
		IMUQ      [3]uint32
	},
) {
	for sample := range samples {
		rawInput := struct {
			Battery  float64
			Altitude float64
			IMU      [3]float64
			Flags    uint8
			Sequence uint16
		}{
			Battery:  sample.Battery,
			Altitude: sample.Altitude,
			IMU:      sample.IMU,
			Flags:    sample.Flags,
			Sequence: sample.SequenceNum & 0xFFF,
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
		priorState.AltitudeQ = qvals.AltitudeQ
		priorState.IMUQ = qvals.IMUQ

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
			true,
			rawSensors,
			quantSensors,
		)
	}
}
