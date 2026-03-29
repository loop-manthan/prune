package engine

import (
	"errors"
	"math"
)

var (
	// ErrInvalidRange indicates max is not greater than min.
	ErrInvalidRange = errors.New("invalid range: max must be greater than min")
	// ErrInvalidBitWidth indicates bit width is outside supported limits.
	ErrInvalidBitWidth = errors.New("invalid bit width: must be in [1, 32]")
)

// Quantize maps a real value to a fixed-width integer with nearest rounding.
// Values outside [min, max] are clamped before mapping.
func Quantize(v, min, max float64, bits uint8) (uint32, error) {
	if bits == 0 || bits > 32 {
		return 0, ErrInvalidBitWidth
	}
	if !(max > min) {
		return 0, ErrInvalidRange
	}

	v = clamp(v, min, max)
	maxCode := maxCode(bits)
	ratio := (v - min) / (max - min)
	q := math.Round(ratio * float64(maxCode))

	if q < 0 {
		q = 0
	}
	if q > float64(maxCode) {
		q = float64(maxCode)
	}

	return uint32(q), nil
}

// Dequantize maps a fixed-width integer back to the real domain.
// Invalid arguments return NaN. q values larger than the representable code
// for bits are clamped to the maximum representable code.
func Dequantize(q uint32, min, max float64, bits uint8) float64 {
	if bits == 0 || bits > 32 {
		return math.NaN()
	}
	if !(max > min) {
		return math.NaN()
	}

	mc := maxCode(bits)
	if q > mc {
		q = mc
	}

	return min + (float64(q)/float64(mc))*(max-min)
}

// QuantizationStep returns the LSB size in the real domain.
// Invalid arguments return NaN.
func QuantizationStep(min, max float64, bits uint8) float64 {
	if bits == 0 || bits > 32 {
		return math.NaN()
	}
	if !(max > min) {
		return math.NaN()
	}

	return (max - min) / float64(maxCode(bits))
}

func maxCode(bits uint8) uint32 {
	if bits == 32 {
		return math.MaxUint32
	}
	return (uint32(1) << bits) - 1
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
