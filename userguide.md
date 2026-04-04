# Prune Source Code Reader Guide

This guide is for engineers who want to understand the source quickly and deeply.
It explains:
- where to start,
- how to move through the codebase,
- what to read first vs later,
- and what each file and function does.

---

## 1) Mental model in one minute

Prune is a telemetry pipeline with this flow:

1. A data source emits `SensorSample` at runtime.
2. Raw values are quantized and (for selected channels) delta-encoded.
3. Bits are packed into a compact frame.
4. CRC-8 is appended for corruption detection.
5. The same frame is decoded immediately for validation/metrics.
6. A Bubble Tea TUI shows compression, errors, latency, and live trends.

Core domains:
- `pkg/telemetry`: schema constants and bit-width contract.
- `internal/engine`: codec primitives and frame encode/decode pipeline.
- `internal/simulator`: synthetic sample generation.
- `internal/source`: pluggable source abstraction (simulator + OpenSky API).
- `internal/tui`: live terminal dashboard and metrics aggregation.
- `cmd/prune`: app composition/wiring.

---

## 2) What to read first (recommended order)

Read in this order for fastest understanding:

1. `README.md`
- Quick intent and runtime flags.
- Note: README still mentions some older phrasing in TUI section; source code is authoritative.

2. `cmd/prune/main.go`
- End-to-end wiring: source selection, codec setup, processing loop, TUI startup.

3. `pkg/telemetry/types.go`
- Single source of truth for ranges, bit widths, and protocol flags.
- Everything in `internal/engine` depends on these constants.

4. `internal/engine/pipeline.go`
- High-level frame model and full encode/decode flow.

5. `internal/engine/{quantize.go,delta.go,packer.go,crc.go}`
- Low-level building blocks used by pipeline.

6. `internal/source/{source.go,simulator.go,opensky.go}`
- Runtime source abstraction and adapter logic.

7. `internal/simulator/{sensors.go,flight_ctrl.go}`
- How synthetic telemetry is generated.

8. `internal/tui/{model.go,update.go,view.go}`
- Metrics lifecycle and rendering details.

9. `internal/engine/*_test.go`
- Behavioral guarantees and edge cases.

---

## 3) What to skip (at first pass)

Safe to skip on first pass:
- `.git/` internals.
- `go1.25.8.linux-amd64.tar.gz` (toolchain artifact).
- `prune` (compiled binary artifact).
- Most test files until you understand production flow.

Read tests later for confidence and invariants, not for initial architecture comprehension.

---

## 4) Directory map and responsibilities

- `cmd/prune/`
  - Entry point and orchestration.
- `pkg/telemetry/`
  - Protocol/schema constants shared by all packages.
- `internal/engine/`
  - Encoding/decoding primitives and frame codec.
- `internal/simulator/`
  - Deterministic/random synthetic telemetry generation.
- `internal/source/`
  - Source interface plus concrete sources.
- `internal/tui/`
  - Bubble Tea model/update/render and metrics visualization.

---

## 5) File-by-file and function-by-function reference

## `cmd/prune/main.go`

### `main()`
- Parses flags:
  - `--source` (`simulator|opensky`)
  - `--seed`
  - `--api-poll`
- Creates cancellable context for all goroutines.
- Builds source via `buildSource`.
- Starts source, with fallback to simulator if source startup fails.
- Creates TUI model and engine codec.
- Initializes prior quantized state used for delta reconstruction.
- Starts background processing loop `processSamples(...)`.
- Runs Bubble Tea program.

### `buildSource(name, seed, pollEvery)`
- Factory:
  - `opensky` -> `source.NewOpenSkySource`
  - default -> `source.NewSimulatorSource`

### `processSamples(model, samples, codec, priorState)`
Main runtime pipeline loop:
- Receives one `SensorSample`.
- Builds normalized raw input struct.
- Measures encode time:
  - `engine.QuantizeAndEncode`
  - `codec.Encode`
- Measures decode time:
  - `codec.Decode`
  - `engine.DecodeAndDequantize`
- Computes absolute per-channel quantization error.
- Estimates original JSON size (fixed 80 bytes heuristic).
- Extracts bitstring from payload for visualization.
- Updates `priorState` with reconstructed quantized values.
- Writes everything into `model.Metrics.RecordSample(...)`.

Why this function matters:
- It is the canonical integration point of source + engine + TUI.

---

## `pkg/telemetry/types.go`

This file defines protocol constants and schema metadata.

### Constants groups
- Battery range/bit width.
- Altitude range/bit width.
- IMU per-axis range/bit width.
- Packet field widths (`VersionBits`, `SequenceBits`, etc.).
- Flag bit semantics (`FlagIMUValid`, `FlagBatteryOk`, etc.).

### `type Channel`
- Metadata for one sensor channel: name, range, bit width, delta capability.

### `var Schema`
- Map of channel identifiers to channel metadata.
- Convenient reference for tooling/extensions; encode path primarily uses direct constants.

---

## `internal/source/source.go`

### `type Source interface`
Contract for telemetry sources:
- `Start(ctx)` starts acquisition/generation.
- `Samples()` returns receive-only sample channel.
- `Name()` returns human-readable source name.

Design intent:
- Decouple pipeline from source implementation.

---

## `internal/source/simulator.go`

Adapter around simulator controller.

### `NewSimulatorSource(seed)`
- Creates source with seeded controller.

### `(*SimulatorSource) Start(ctx)`
- Starts controller loop.

### `(*SimulatorSource) Samples()`
- Exposes controller channel.

### `(*SimulatorSource) Name()`
- Returns `"simulator"`.

---

## `internal/source/opensky.go`

OpenSky API-backed source adapter.

### `NewOpenSkySource(pollEvery)`
- Validates/defaults poll interval.
- Configures HTTP client timeout.
- Creates buffered sample channel.

### `(*OpenSkySource) Start(ctx)`
- Performs immediate `pollOnce` (fail-fast behavior).
- Launches:
  - `pollLoop` for periodic API refresh.
  - `emitLoop` for smooth sample emission (~20 Hz).

### `(*OpenSkySource) Samples()`
- Returns sample channel.

### `(*OpenSkySource) Name()`
- Returns `"opensky"`.

### `(*OpenSkySource) pollLoop(ctx)`
- Ticker-driven API polling at configured interval.
- Ignores transient poll errors (keeps running).

### `(*OpenSkySource) emitLoop(ctx)`
- Ticker-driven output loop.
- Reads latest snapshot under read lock.
- Sets fresh timestamp and sequence.
- Emits to channel until context cancellation.
- Closes channel on exit.

### `(*OpenSkySource) pollOnce(ctx)`
- Performs HTTP GET and status validation.
- Decodes JSON payload.
- Adapts first valid state row via `adaptOpenSky`.
- Writes latest sample under mutex.

### `(*OpenSkySource) nextSeq()`
- Thread-safe sequence increment.

### `adaptOpenSky(data)`
- Selects usable aircraft row.
- Extracts altitude, vertical rate, speed.
- Maps into project schema ranges:
  - altitude -> clamped altitude
  - velocity -> IMU X proxy
  - vertical rate -> IMU Z proxy
- Sets flags and synthetic battery value.
- Returns adapted `SensorSample`.

### `toFloat(row, idx)`
- Safe typed extraction from dynamic JSON row.

### `clamp(v, lo, hi)`
- Local float clamp helper.

Important behavior:
- OpenSky sample cadence and API cadence are decoupled.
- Last fetched state is re-emitted at stable local tick interval.

---

## `internal/simulator/sensors.go`

Pure data models for simulator.

### `type SensorSample`
- Canonical runtime sample payload used across source + pipeline.

### `type AltitudeState`
- Parameters for altitude sine model.

### `type BatteryState`
- Parameters for battery decay/spike model.

### `type IMUState`
- Parameters/state for random walk IMU.

---

## `internal/simulator/flight_ctrl.go`

Generates synthetic telemetry stream.

### `NewController(seed)`
- Initializes simulator states and RNG.
- `seed == 0` means non-deterministic random seed.

### `(*Controller) Start(ctx)`
- Starts 50 ms ticker and run loop.

### `(*Controller) Samples()`
- Returns output sample channel.

### `(*Controller) run(ctx)`
- Main ticker loop.
- Computes elapsed time and emits generated sample.

### `(*Controller) generateSample(elapsed, ts)`
Signal model details:
- Altitude = base + sine + Gaussian noise.
- Battery = linear discharge + sparse downward spikes.
- IMU = 3-axis random walk with Gaussian steps + clamp to sensor bounds.
- Sets readiness/battery flags.
- Assigns sequence number and increments it.

Why this matters:
- Defines expected telemetry dynamics that delta encoding should exploit.

---

## `internal/engine/quantize.go`

Real-value <-> fixed-width integer mapping.

### `Quantize(v, min, max, bits)`
- Validates args.
- Clamps real value to domain.
- Maps to `[0, 2^bits-1]` with nearest rounding.

### `Dequantize(q, min, max, bits)`
- Validates args, clamps code, maps back to real domain.
- Returns `NaN` for invalid args.

### `QuantizationStep(min, max, bits)`
- Returns LSB size in real units.

### `maxCode(bits)`
- Max code helper for bit width.

### `clamp(v, min, max)`
- Internal clamp helper.

---

## `internal/engine/delta.go`

Small-delta optimization with keyframe fallback.

### `EncodeDelta(curr, prev)`
- If delta in `[-8,7]`:
  - returns delta mode (4-bit payload).
- Else:
  - returns keyframe mode with full-width payload (default 32-bit form).

### `EncodeDeltaForWidth(curr, prev, keyframeWidth)`
- Width-aware variant for fixed packet layouts.
- Keyframe payload masked to requested width.

### `DecodeDelta(isKeyframe, payload, prev)`
- Keyframe: payload is current value.
- Delta mode: decodes signed 4-bit delta and applies saturating add.

### `encodeDelta4(d)` / `decodeDelta4(v)`
- Two's-complement packing/unpacking for 4-bit deltas.

### `addSaturatingSigned(base, delta)`
- Prevents uint underflow/overflow during reconstruction.

---

## `internal/engine/packer.go`

Bit-level serializer/deserializer.

### `type BitBuffer`
- Holds byte buffer and independent read/write bit cursors.

### `NewBitBuffer()`
- Empty writer.

### `NewBitBufferFromBytes(data)`
- Reader initialized from existing bytes.

### `(*BitBuffer) WriteBits(value, width)`
- Appends lower `width` bits, MSB-first.

### `(*BitBuffer) ReadBits(width)`
- Reads next `width` bits from read cursor.

### `(*BitBuffer) Bytes()`
- Copy of packed bytes.

### `(*BitBuffer) BitString()`
- Human-readable `0/1` representation of written bits.

### `(*BitBuffer) Reset()`
- Clears buffer and cursors.

### `(*BitBuffer) RewindRead()`
- Resets read cursor only.

### `(*BitBuffer) WrittenBits()`
- Total written bits.

### `(*BitBuffer) ReadBitsOffset()`
- Current read cursor position.

### `lowerBitMask(width)`
- Helper bitmask for truncating input value.

---

## `internal/engine/crc.go`

CRC-8 integrity checks.

### `CRC8(data)`
- Computes CRC-8 with polynomial `0x07`, init `0x00`, no final XOR.

### `AppendCRC(frame)`
- Appends CRC byte to payload.

### `ValidateCRC(frame)`
- Checks trailing CRC against payload.

---

## `internal/engine/pipeline.go`

High-level packet format and end-to-end codec behavior.

### `type EncodedFrame`
Holds:
- Header fields (version, sequence, flags).
- Quantized payload fields.
- Mode flags for delta/keyframe interpretation.
- Previous quantized state for decode reconstruction.
- Serialized bytes.

### `type FrameCodec`
- Owns reusable write/read buffers.

### `NewFrameCodec()`
- Constructs codec instance.

### `(*FrameCodec) Encode(frame)`
- Writes header bits.
- Writes battery absolute quantized value.
- Writes altitude mode + payload.
- Writes each IMU axis mode + payload.
- Appends CRC.
- Stores serialized bytes back into frame.

### `(*FrameCodec) Decode(data)`
- CRC validation.
- Reads all fields in exact bit order.
- Reconstructs quantized payload and modes into `EncodedFrame`.

### `QuantizeAndEncode(raw, priorState)`
- Quantizes all channels using schema ranges.
- Chooses delta/keyframe for altitude and each IMU axis.
- Produces `EncodedFrame` with payload values in quantized domain.

### `DecodeAndDequantize(frame)`
- Reconstructs current quantized values from modes + previous values.
- Dequantizes to real units.
- Returns both real-valued outputs and quantized values.

Important subtlety:
- During `Encode`, fields like `AltitudeQ` may carry either delta payload or keyframe payload depending on mode; `DecodeAndDequantize` resolves to true current quantized values.

---

## `internal/tui/model.go`

Metrics state and statistics calculations.

### `type MetricsCollector`
Tracks:
- Throughput/sample counters.
- Compression bytes.
- CRC counts.
- Latencies (last + smoothed average).
- Quantization error accumulation for RMSE.
- Latest raw/reconstructed sensor values.
- Short history windows (altitude, battery) for graphing.
- Recent bitstrings for bit-level inspection.

### `NewMetricsCollector()`
- Initializes ring buffers and history capacities.

### `(*MetricsCollector) RecordSample(...)`
- Ingests one processed sample's metrics.
- Updates all counters and latest values.
- Updates exponential moving averages for latency.
- Accumulates squared errors for RMSE.
- Maintains ring buffers/histories.

### `CompressionRatio()`
- `jsonBytes / packedBytes`.

### `SavingsPercent()`
- `100 * (1 - packed/json)`.

### `CRCSuccessRate()`
- Success ratio over CRC attempts.

### `RMSE()`
- Per-channel root mean square quantization error.

### `type TUIModel`
- Bubble Tea state: metrics, dimensions, source name, time counters.

### `NewModel(width, height, sourceName)`
- Creates dashboard state.

---

## `internal/tui/update.go`

Bubble Tea update loop and app lifecycle.

### `type TickMsg`
- Periodic refresh message payload.

### `(*TUIModel) Init()`
- Starts periodic ticking.

### `(*TUIModel) Update(msg)`
- Handles:
  - window resize updates,
  - quit keys (`q`, `ctrl+c`),
  - periodic tick increments and runtime update.

### `(*TUIModel) View()`
- Delegates to render function.

### `getTick()`
- Schedules next `TickMsg` at 100 ms cadence.

---

## `internal/tui/view.go`

Rendering and visual analytics.

### `(*TUIModel) Render()`
- Creates 3-panel sideways-T layout.
- Left: comparisons.
- Top-right: system state explanations/metrics.
- Bottom-right: live altitude/battery plots.

### `renderComparisons(m)`
- Compression ratio/savings/CRC.
- Raw vs reconstructed battery and altitude.
- RMSE, latency, byte counters.

### `renderSystemState(m)`
- Derived operational indicators:
  - sample rate,
  - altitude trend,
  - battery status,
  - IMU stability,
  - frame bit estimate.

### `trendIndicator(history)`
- Rising/falling/stable based on midpoint-vs-latest comparison.

### `batteryStatus(v)`
- Qualitative battery class (`good|ok|low`).

### `imuIndicator(rmse)`
- Qualitative IMU noise label.

### `estimateFrameBits(m)`
- Average bits per packed frame.

### `renderAltitudeBatteryGraph(width, height, m)`
- Draws altitude and battery sections using historical windows.

### `renderLineGraph(history, width, height, minVal, maxVal)`
- Normalizes history into `[0,1]`, plots ASCII points on grid.

---

## 6) Test files: when and why to read

Read tests after production flow is clear.

- `internal/engine/quantize_test.go`
  - Edge handling, monotonicity, error bounds.
- `internal/engine/delta_test.go`
  - Delta boundaries, fallback behavior, reconstruction.
- `internal/engine/packer_test.go`
  - Bit alignment, masks, cursor behavior, randomized roundtrip.
- `internal/engine/crc_test.go`
  - Golden vectors and corruption detection.
- `internal/engine/phase1_integration_test.go`
  - Quantization+packing integration sanity.
- `internal/engine/pipeline_test.go`
  - End-to-end encode/decode and simulator integration.

Use tests as executable specification for corner cases.

---

## 7) How to trace one sample end-to-end (practical exercise)

Do this once and the architecture becomes obvious:

1. In `main.go`, locate one loop iteration in `processSamples`.
2. Follow call into `QuantizeAndEncode`.
3. Follow `FrameCodec.Encode` bit write order.
4. Follow `FrameCodec.Decode` read order.
5. Follow `DecodeAndDequantize` reconstruction logic.
6. Return to `RecordSample` in TUI model.
7. See where `renderComparisons` and `renderAltitudeBatteryGraph` display those values.

This gives a complete mental path from source sample to rendered dashboard pixel.

---

## 8) Common pitfalls while reading

- Mode semantics:
  - In this code, `AltMode`/`IMUModes[axis] == true` means keyframe mode.
- Payload interpretation:
  - Quantized payload fields may hold delta payloads before reconstruction.
- Prior-state dependency:
  - Correct decode of delta payloads requires previous quantized values.
- Source cadence mismatch:
  - OpenSky API poll interval is slower than local emit interval by design.
- README drift:
  - UI wording in README can lag behind current source; trust code.

---

## 9) If you want to contribute safely

Start with these low-risk tasks:
- Add comments around bit packing order in `pipeline.go`.
- Add tests for OpenSky adaptation edge rows.
- Add benchmarks for `BitBuffer.WriteBits` and `ReadBits`.
- Clarify README sections that are now stale relative to TUI behavior.

Then move to medium-risk tasks:
- Add configurable graph history size.
- Improve frame-size estimation and reporting.
- Add strict source health indicators in TUI.

---

## 10) Quick index of production files

- `cmd/prune/main.go`
- `pkg/telemetry/types.go`
- `internal/source/source.go`
- `internal/source/simulator.go`
- `internal/source/opensky.go`
- `internal/simulator/sensors.go`
- `internal/simulator/flight_ctrl.go`
- `internal/engine/quantize.go`
- `internal/engine/delta.go`
- `internal/engine/packer.go`
- `internal/engine/crc.go`
- `internal/engine/pipeline.go`
- `internal/tui/model.go`
- `internal/tui/update.go`
- `internal/tui/view.go`

This index is the minimum production surface for architecture understanding.