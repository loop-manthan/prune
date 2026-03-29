package engine

import (
	"math/rand"
	"testing"
)

func TestBitBufferMixedWidthRoundtrip(t *testing.T) {
	b := NewBitBuffer()

	inputs := []struct {
		value uint64
		width uint8
	}{
		{0b10101, 5},
		{0b11100101101, 11},
		{0b1011011, 7},
		{0b1101100101011, 13},
	}

	for _, in := range inputs {
		if err := b.WriteBits(in.value, in.width); err != nil {
			t.Fatalf("WriteBits(%d,%d): %v", in.value, in.width, err)
		}
	}

	b.RewindRead()
	for i, in := range inputs {
		got, err := b.ReadBits(in.width)
		if err != nil {
			t.Fatalf("ReadBits(%d) at %d: %v", in.width, i, err)
		}
		if got != in.value {
			t.Fatalf("roundtrip[%d]: got %b, want %b", i, got, in.value)
		}
	}
}

func TestBitBufferBoundaryCrossingEveryOffset(t *testing.T) {
	for pad := uint8(0); pad < 8; pad++ {
		b := NewBitBuffer()

		if pad > 0 {
			if err := b.WriteBits(0, pad); err != nil {
				t.Fatalf("pad WriteBits failed at pad=%d: %v", pad, err)
			}
		}

		want := uint64(0b1010101010111) // 13 bits crosses byte boundary at all pads
		if err := b.WriteBits(want, 13); err != nil {
			t.Fatalf("WriteBits 13 failed at pad=%d: %v", pad, err)
		}

		b.RewindRead()
		if pad > 0 {
			if _, err := b.ReadBits(pad); err != nil {
				t.Fatalf("pad ReadBits failed at pad=%d: %v", pad, err)
			}
		}

		got, err := b.ReadBits(13)
		if err != nil {
			t.Fatalf("ReadBits 13 failed at pad=%d: %v", pad, err)
		}
		if got != want {
			t.Fatalf("boundary pad=%d: got %b, want %b", pad, got, want)
		}
	}
}

func TestBitBufferWriteMasksLowerBits(t *testing.T) {
	b := NewBitBuffer()

	if err := b.WriteBits(0xFFFF, 6); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	b.RewindRead()
	got, err := b.ReadBits(6)
	if err != nil {
		t.Fatalf("ReadBits: %v", err)
	}
	if got != 0x3F {
		t.Fatalf("masked value: got %d, want %d", got, 0x3F)
	}
}

func TestBitBufferErrors(t *testing.T) {
	b := NewBitBuffer()

	if err := b.WriteBits(1, 0); err == nil {
		t.Fatal("expected error for width=0 write")
	}
	if err := b.WriteBits(1, 65); err == nil {
		t.Fatal("expected error for width>64 write")
	}
	if _, err := b.ReadBits(0); err == nil {
		t.Fatal("expected error for width=0 read")
	}
	if _, err := b.ReadBits(65); err == nil {
		t.Fatal("expected error for width>64 read")
	}

	if err := b.WriteBits(0b101, 3); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	if _, err := b.ReadBits(4); err == nil {
		t.Fatal("expected read past end error")
	}
}

func TestBitBufferResetAndBytesCopy(t *testing.T) {
	b := NewBitBuffer()
	if err := b.WriteBits(0xAB, 8); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	data := b.Bytes()
	if len(data) != 1 || data[0] != 0xAB {
		t.Fatalf("Bytes mismatch: got %v", data)
	}
	data[0] = 0x00
	if b.Bytes()[0] != 0xAB {
		t.Fatal("Bytes() must return a copy, not internal slice")
	}

	b.Reset()
	if b.WrittenBits() != 0 {
		t.Fatalf("after Reset, WrittenBits=%d want 0", b.WrittenBits())
	}
	if b.ReadBitsOffset() != 0 {
		t.Fatalf("after Reset, ReadBitsOffset=%d want 0", b.ReadBitsOffset())
	}
	if len(b.Bytes()) != 0 {
		t.Fatalf("after Reset, expected empty bytes")
	}
}

func TestBitBufferBitString(t *testing.T) {
	b := NewBitBuffer()
	if err := b.WriteBits(0b101, 3); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}
	if err := b.WriteBits(0b11, 2); err != nil {
		t.Fatalf("WriteBits: %v", err)
	}

	got := b.BitString()
	if got != "10111" {
		t.Fatalf("BitString: got %q, want %q", got, "10111")
	}
}

func TestBitBufferRandomizedRoundtrip(t *testing.T) {
	rng := rand.New(rand.NewSource(99))

	for iter := 0; iter < 200; iter++ {
		b := NewBitBuffer()

		type pair struct {
			v uint64
			w uint8
		}
		pairs := make([]pair, 0, 64)

		n := 10 + rng.Intn(50)
		for i := 0; i < n; i++ {
			w := uint8(1 + rng.Intn(64))
			v := rng.Uint64() & lowerBitMask(w)
			pairs = append(pairs, pair{v: v, w: w})
			if err := b.WriteBits(v, w); err != nil {
				t.Fatalf("iter %d write %d failed: %v", iter, i, err)
			}
		}

		b.RewindRead()
		for i, p := range pairs {
			got, err := b.ReadBits(p.w)
			if err != nil {
				t.Fatalf("iter %d read %d failed: %v", iter, i, err)
			}
			if got != p.v {
				t.Fatalf("iter %d pair %d mismatch: got %d want %d (w=%d)", iter, i, got, p.v, p.w)
			}
		}

		if b.ReadBitsOffset() != b.WrittenBits() {
			t.Fatalf("iter %d cursor mismatch: read=%d write=%d", iter, b.ReadBitsOffset(), b.WrittenBits())
		}
	}
}
