package engine

import (
	"math"
	"math/rand"
	"testing"
)

func TestQuantizeEdgesAndClamp(t *testing.T) {
	const (
		min  = 10.0
		max  = 15.0
		bits = uint8(6)
	)

	qMin, err := Quantize(min, min, max, bits)
	if err != nil {
		t.Fatalf("Quantize(min): unexpected error: %v", err)
	}
	if qMin != 0 {
		t.Fatalf("Quantize(min): got %d, want 0", qMin)
	}

	qMax, err := Quantize(max, min, max, bits)
	if err != nil {
		t.Fatalf("Quantize(max): unexpected error: %v", err)
	}
	if qMax != 63 {
		t.Fatalf("Quantize(max): got %d, want 63", qMax)
	}

	qLow, err := Quantize(min-100, min, max, bits)
	if err != nil {
		t.Fatalf("Quantize(low clamp): unexpected error: %v", err)
	}
	if qLow != 0 {
		t.Fatalf("Quantize(low clamp): got %d, want 0", qLow)
	}

	qHigh, err := Quantize(max+100, min, max, bits)
	if err != nil {
		t.Fatalf("Quantize(high clamp): unexpected error: %v", err)
	}
	if qHigh != 63 {
		t.Fatalf("Quantize(high clamp): got %d, want 63", qHigh)
	}
}

func TestQuantizeInvalidArguments(t *testing.T) {
	if _, err := Quantize(1.0, 5.0, 5.0, 6); err == nil {
		t.Fatal("expected error for equal min/max")
	}
	if _, err := Quantize(1.0, 6.0, 5.0, 6); err == nil {
		t.Fatal("expected error for min > max")
	}
	if _, err := Quantize(1.0, 0.0, 1.0, 0); err == nil {
		t.Fatal("expected error for bits == 0")
	}
	if _, err := Quantize(1.0, 0.0, 1.0, 33); err == nil {
		t.Fatal("expected error for bits > 32")
	}
}

func TestQuantizationStep(t *testing.T) {
	got := QuantizationStep(10.0, 15.0, 6)
	want := 5.0 / 63.0
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("QuantizationStep: got %.15f, want %.15f", got, want)
	}

	if !math.IsNaN(QuantizationStep(1.0, 1.0, 6)) {
		t.Fatal("expected NaN for invalid range")
	}
	if !math.IsNaN(QuantizationStep(0.0, 1.0, 0)) {
		t.Fatal("expected NaN for bits == 0")
	}
}

func TestDequantizeInvalidArguments(t *testing.T) {
	if !math.IsNaN(Dequantize(0, 1.0, 1.0, 6)) {
		t.Fatal("expected NaN for invalid range")
	}
	if !math.IsNaN(Dequantize(0, 0.0, 1.0, 0)) {
		t.Fatal("expected NaN for bits == 0")
	}
}

func TestDequantizeClampsCode(t *testing.T) {
	got := Dequantize(1000, 10.0, 15.0, 6)
	want := 15.0
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("Dequantize clamp: got %.15f, want %.15f", got, want)
	}
}

func TestQuantizeDequantizeErrorBoundRandomized(t *testing.T) {
	const (
		min     = 0.0
		max     = 5000.0
		bits    = uint8(14)
		samples = 10000
	)

	step := QuantizationStep(min, max, bits)
	limit := step/2 + 1e-12
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < samples; i++ {
		v := min + rng.Float64()*(max-min)
		q, err := Quantize(v, min, max, bits)
		if err != nil {
			t.Fatalf("sample %d: unexpected quantize error: %v", i, err)
		}
		r := Dequantize(q, min, max, bits)
		errAbs := math.Abs(v - r)
		if errAbs > limit {
			t.Fatalf("sample %d: |error|=%.15f exceeds %.15f (v=%.15f q=%d r=%.15f)", i, errAbs, limit, v, q, r)
		}
	}
}

func TestQuantizeMonotonicity(t *testing.T) {
	const (
		min  = 10.0
		max  = 15.0
		bits = uint8(6)
	)

	prev := uint32(0)
	for i := 0; i <= 1000; i++ {
		v := min + (float64(i)/1000.0)*(max-min)
		q, err := Quantize(v, min, max, bits)
		if err != nil {
			t.Fatalf("unexpected quantize error at i=%d: %v", i, err)
		}
		if i > 0 && q < prev {
			t.Fatalf("non-monotonic quantization at i=%d: prev=%d curr=%d", i, prev, q)
		}
		prev = q
	}
}