package engine

import "errors"

const (
	// DeltaWidth is the encoded width (bits) for in-range deltas.
	DeltaWidth uint8 = 4
	// DeltaMin is the minimum signed delta representable in DeltaWidth bits.
	DeltaMin int32 = -8
	// DeltaMax is the maximum signed delta representable in DeltaWidth bits.
	DeltaMax int32 = 7
)

var (
	// ErrInvalidKeyframeWidth indicates a keyframe bit width outside [1, 32].
	ErrInvalidKeyframeWidth = errors.New("invalid keyframe width: must be in [1, 32]")
)

// EncodeDelta encodes curr relative to prev.
//
// Behavior:
// - If curr-prev is in [-8, 7], returns delta mode (isKeyframe=false),
//   4-bit two's-complement payload, width=4.
// - Otherwise returns keyframe mode (isKeyframe=true), payload=curr, width=32.
//
// For schema-sized keyframes (e.g. 14-bit altitude), use EncodeDeltaForWidth.
func EncodeDelta(curr, prev uint32) (isKeyframe bool, payload uint32, width uint8) {
	d := int64(curr) - int64(prev)
	if d >= int64(DeltaMin) && d <= int64(DeltaMax) {
		return false, encodeDelta4(int32(d)), DeltaWidth
	}
	return true, curr, 32
}

// EncodeDeltaForWidth is a width-aware variant of EncodeDelta for packet layouts
// where keyframes use a known fixed bit width (for example, 14-bit altitude).
//
// On keyframe fallback, payload is masked to keyframeWidth bits and width is
// returned as keyframeWidth.
func EncodeDeltaForWidth(curr, prev uint32, keyframeWidth uint8) (isKeyframe bool, payload uint32, width uint8, err error) {
	if keyframeWidth == 0 || keyframeWidth > 32 {
		return false, 0, 0, ErrInvalidKeyframeWidth
	}

	d := int64(curr) - int64(prev)
	if d >= int64(DeltaMin) && d <= int64(DeltaMax) {
		return false, encodeDelta4(int32(d)), DeltaWidth, nil
	}

	return true, curr & maxCode(keyframeWidth), keyframeWidth, nil
}

// DecodeDelta decodes either delta mode or keyframe mode.
//
// For delta mode, payload is interpreted as 4-bit two's-complement and applied
// to prev with saturating arithmetic to avoid uint32 underflow/overflow.
func DecodeDelta(isKeyframe bool, payload uint32, prev uint32) (curr uint32) {
	if isKeyframe {
		return payload
	}

	d := decodeDelta4(payload)
	return addSaturatingSigned(prev, d)
}

func encodeDelta4(d int32) uint32 {
	return uint32(d) & 0x0F
}

func decodeDelta4(v uint32) int32 {
	v &= 0x0F
	if v&0x08 != 0 {
		return int32(v) - 16
	}
	return int32(v)
}

func addSaturatingSigned(base uint32, delta int32) uint32 {
	if delta >= 0 {
		du := uint32(delta)
		if du > ^uint32(0)-base {
			return ^uint32(0)
		}
		return base + du
	}

	du := uint32(-delta)
	if du > base {
		return 0
	}
	return base - du
}
