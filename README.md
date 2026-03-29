# Prune

Prune is a high-performance telemetry compression engine in Go focused on solving fat telemetry in constrained aerospace links.

## Project Goals

- Compress wide-range sensor data using fixed-width quantization.
- Bit-stitch odd-width fields into dense binary packets with zero wasted space.
- Use delta encoding for slowly changing channels.
- Simulate realistic flight-controller telemetry streams without hardware.
- Track transmission integrity with CRC-8 checksums.

## Running

```bash
./prune
```

Interactive dashboard with keyboard controls:
- **1-6**: Jump to specific metric view
- **← →**: Navigate between views
- **q**: Quit

