# Prune

Prune is a telemetry compression engine in Go for constrained links.

## Project Goals

- Compress wide-range sensor data using fixed-width quantization.
- Pack odd-width fields into compact binary frames.
- Use delta encoding for slowly changing channels.
- Support simulator and API-backed real-time data sources.
- Detect corruption with CRC-8 checksums.

## Running

```bash
./prune
```

Choose a source:

```bash
./prune --source simulator --seed 42
./prune --source opensky --api-poll 1s
```

If OpenSky is unavailable, Prune automatically falls back to the simulator.

## TUI Layout

The terminal UI is now a single screen split into three parts in a sideways-T style:

- Part 1 (left): live comparisons (raw vs reconstructed values, compression, latency, CRC)
- Part 2 (top-right): plain-English science and logic notes
- Part 3 (bottom-right): live ASCII visualization (animated sine wave)

Controls:
- q: quit
