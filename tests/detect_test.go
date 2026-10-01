//go:build wasm

package tests

import (
	"testing"

	"webtyp.com/device"
)

func TestDetect_InBrowser(t *testing.T) {
	p, err := device.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if p.Cores <= 0 {
		t.Fatalf("expected Cores > 0, got %d", p.Cores)
	}
	if !p.SIMD {
		t.Fatalf("expected SIMD == true in browser environment")
	}
}

func TestPersist_NoError(t *testing.T) {
	_, err := device.Persist()
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}
}

func TestBench_CountsRuns(t *testing.T) {
	n := 0
	rate := device.Bench(func() {
		n++
	}, 20)

	if n < 4 {
		t.Fatalf("expected n >= 4 (1 warm-up + at least 3 runs), got %d", n)
	}
	if rate <= 0 {
		t.Fatalf("expected Rate > 0, got %f", rate)
	}
}

func TestBench_ConsumerShaped(t *testing.T) {
	p, err := device.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	a := make([]float32, 64*64)
	b := make([]float32, 64*64)
	c := make([]float32, 64*64)
	for i := range a {
		a[i] = float32(i % 10)
		b[i] = float32(i % 5)
	}

	kernel := func() {
		for i := 0; i < 64; i++ {
			for j := 0; j < 64; j++ {
				var sum float32
				for k := 0; k < 64; k++ {
					sum += a[i*64+k] * b[k*64+j]
				}
				c[i*64+j] = sum
			}
		}
	}

	r := device.Bench(kernel, 50)
	req := device.Requirement{
		MinTier: device.TierPlain,
		MinRate: 1,
	}

	shortfalls := req.Check(p, r)
	for _, sf := range shortfalls {
		if sf.Blocking() && sf != device.ShortSecure && sf != device.ShortSpace {
			t.Fatalf("unexpected blocking shortfall: %v", sf)
		}
	}
}
