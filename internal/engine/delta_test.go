package engine

import (
	"math/rand"
	"testing"
)

func TestEncodeDeltaInRangeUsesDeltaMode(t *testing.T) {
	prev := uint32(100)
	curr := uint32(95) // delta -5, in range

	isKeyframe, payload, width := EncodeDelta(curr, prev)
	if isKeyframe {
		t.Fatal("expected delta mode, got keyframe")
	}
	if width != DeltaWidth {
		t.Fatalf("delta width: got %d, want %d", width, DeltaWidth)
	}

	got := DecodeDelta(isKeyframe, payload, prev)
	if got != curr {
		t.Fatalf("decoded curr: got %d, want %d", got, curr)
	}
}

func TestEncodeDeltaOverflowUsesKeyframe(t *testing.T) {
	prev := uint32(100)
	curr := uint32(120) // delta +20, out of range

	isKeyframe, payload, width := EncodeDelta(curr, prev)
	if !isKeyframe {
		t.Fatal("expected keyframe mode, got delta")
	}
	if width != 32 {
		t.Fatalf("keyframe width: got %d, want %d", width, 32)
	}

	got := DecodeDelta(isKeyframe, payload, prev)
	if got != curr {
		t.Fatalf("decoded keyframe curr: got %d, want %d", got, curr)
	}
}

func TestEncodeDeltaBoundaries(t *testing.T) {
	prev := uint32(1000)
	cases := []struct {
		name string
		curr uint32
		key  bool
	}{
		{name: "min in-range", curr: uint32(int64(prev) + int64(DeltaMin)), key: false},
		{name: "max in-range", curr: uint32(int64(prev) + int64(DeltaMax)), key: false},
		{name: "below min", curr: uint32(int64(prev) + int64(DeltaMin-1)), key: true},
		{name: "above max", curr: uint32(int64(prev) + int64(DeltaMax+1)), key: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isKeyframe, payload, _ := EncodeDelta(tc.curr, prev)
			if isKeyframe != tc.key {
				t.Fatalf("isKeyframe=%v, want %v", isKeyframe, tc.key)
			}
			got := DecodeDelta(isKeyframe, payload, prev)
			if got != tc.curr {
				t.Fatalf("decoded curr: got %d, want %d", got, tc.curr)
			}
		})
	}
}

func TestEncodeDeltaForWidth(t *testing.T) {
	prev := uint32(100)
	curr := uint32(120)

	isKeyframe, payload, width, err := EncodeDeltaForWidth(curr, prev, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isKeyframe {
		t.Fatal("expected keyframe")
	}
	if width != 14 {
		t.Fatalf("keyframe width: got %d, want 14", width)
	}
	if payload != curr {
		t.Fatalf("payload mismatch: got %d, want %d", payload, curr)
	}
}

func TestEncodeDeltaForWidthInvalidWidth(t *testing.T) {
	if _, _, _, err := EncodeDeltaForWidth(10, 9, 0); err == nil {
		t.Fatal("expected error for keyframeWidth=0")
	}
	if _, _, _, err := EncodeDeltaForWidth(10, 9, 33); err == nil {
		t.Fatal("expected error for keyframeWidth>32")
	}
}

func TestDecodeDeltaSaturates(t *testing.T) {
	// Corrupt payload decodes as -1 (0b1111) and would underflow without saturation.
	if got := DecodeDelta(false, 0x0F, 0); got != 0 {
		t.Fatalf("underflow saturation failed: got %d, want 0", got)
	}
}

func TestDeltaStateMachineLongSequence(t *testing.T) {
	seq := []uint32{1000, 1002, 1005, 1003, 1004, 1030, 1031, 1029, 900, 905, 906}

	var prev uint32
	for i, curr := range seq {
		if i == 0 {
			prev = curr
			continue
		}

		isKeyframe, payload, _ := EncodeDelta(curr, prev)
		recon := DecodeDelta(isKeyframe, payload, prev)
		if recon != curr {
			t.Fatalf("index %d reconstruction mismatch: got %d, want %d", i, recon, curr)
		}
		prev = recon
	}
}

func TestDeltaRandomizedReconstruction(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for iter := 0; iter < 300; iter++ {
		prev := uint32(rng.Intn(1 << 14))

		for i := 0; i < 120; i++ {
			step := rng.Intn(35) - 17 // intentionally outside [-8,7] often
			currSigned := int64(prev) + int64(step)
			if currSigned < 0 {
				currSigned = 0
			}
			if currSigned > (1<<14)-1 {
				currSigned = (1 << 14) - 1
			}
			curr := uint32(currSigned)

			isKeyframe, payload, _ := EncodeDelta(curr, prev)
			recon := DecodeDelta(isKeyframe, payload, prev)
			if recon != curr {
				t.Fatalf("iter %d i %d mismatch: got %d want %d", iter, i, recon, curr)
			}
			prev = recon
		}
	}
}
