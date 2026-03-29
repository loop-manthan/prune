package engine

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestCRC8GoldenVector123456789(t *testing.T) {
	got := CRC8([]byte("123456789"))
	const want = uint8(0xF4) // Standard CRC-8 (poly 0x07, init 0x00, xorout 0x00)
	if got != want {
		t.Fatalf("CRC8(123456789): got 0x%02X, want 0x%02X", got, want)
	}
}

func TestCRC8EmptyAndDeterministic(t *testing.T) {
	if got := CRC8(nil); got != 0x00 {
		t.Fatalf("CRC8(nil): got 0x%02X, want 0x00", got)
	}

	payload := []byte{0x00, 0x01, 0x02, 0xAB, 0xCD, 0xEF}
	c1 := CRC8(payload)
	c2 := CRC8(payload)
	if c1 != c2 {
		t.Fatalf("CRC8 non-deterministic: %02X vs %02X", c1, c2)
	}
}

func TestAppendCRCAndValidate(t *testing.T) {
	orig := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	origCopy := append([]byte(nil), orig...)

	framed := AppendCRC(orig)
	if len(framed) != len(orig)+1 {
		t.Fatalf("framed len: got %d, want %d", len(framed), len(orig)+1)
	}
	if !bytes.Equal(orig, origCopy) {
		t.Fatal("AppendCRC mutated input payload")
	}
	if !ValidateCRC(framed) {
		t.Fatal("ValidateCRC should pass for freshly appended CRC")
	}

	expectedCRC := CRC8(orig)
	if framed[len(framed)-1] != expectedCRC {
		t.Fatalf("trailing crc: got 0x%02X, want 0x%02X", framed[len(framed)-1], expectedCRC)
	}
}

func TestValidateCRCErrors(t *testing.T) {
	if ValidateCRC(nil) {
		t.Fatal("ValidateCRC(nil) should be false")
	}
	if ValidateCRC([]byte{}) {
		t.Fatal("ValidateCRC(empty) should be false")
	}
}

func TestCRCDetectsSingleBitCorruption(t *testing.T) {
	payload := []byte{0x10, 0x20, 0x30, 0x40, 0x55, 0xAA}
	framed := AppendCRC(payload)
	if !ValidateCRC(framed) {
		t.Fatal("baseline framed payload should validate")
	}

	for bit := 0; bit < len(framed)*8; bit++ {
		corrupt := append([]byte(nil), framed...)
		byteIdx := bit / 8
		bitIdx := uint(bit % 8)
		corrupt[byteIdx] ^= 1 << bitIdx
		if ValidateCRC(corrupt) {
			t.Fatalf("single-bit corruption undetected at bit %d", bit)
		}
	}
}

func TestCRCRandomizedRoundtripAndCorruption(t *testing.T) {
	rng := rand.New(rand.NewSource(20260329))

	for i := 0; i < 500; i++ {
		n := 1 + rng.Intn(256)
		payload := make([]byte, n)
		for j := range payload {
			payload[j] = byte(rng.Intn(256))
		}

		framed := AppendCRC(payload)
		if !ValidateCRC(framed) {
			t.Fatalf("iteration %d: framed payload should validate", i)
		}

		// Flip one random payload bit (not the CRC byte) and ensure failure.
		byteIdx := rng.Intn(len(payload))
		bitIdx := uint(rng.Intn(8))
		corrupt := append([]byte(nil), framed...)
		corrupt[byteIdx] ^= 1 << bitIdx
		if ValidateCRC(corrupt) {
			t.Fatalf("iteration %d: corruption should fail validation", i)
		}
	}
}
