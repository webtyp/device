package tests

import (
	"testing"

	"webtyp.com/device"
)

func TestProfile_Free_NeverNegative(t *testing.T) {
	p := device.Profile{Quota: 100, Usage: 150}
	if free := p.Free(); free != 0 {
		t.Fatalf("expected Free 0, got %d", free)
	}

	p2 := device.Profile{Quota: 200, Usage: 50}
	if free := p2.Free(); free != 150 {
		t.Fatalf("expected Free 150, got %d", free)
	}
}

func TestProfile_Tier(t *testing.T) {
	tests := []struct {
		name   string
		p      device.Profile
		want   device.Tier
	}{
		{"none", device.Profile{}, device.TierPlain},
		{"simd", device.Profile{SIMD: true}, device.TierSIMD},
		{"webgpu", device.Profile{WebGPU: true}, device.TierWebGPU},
		{"both", device.Profile{SIMD: true, WebGPU: true}, device.TierWebGPU},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Tier(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCheck_MetIsEmpty(t *testing.T) {
	p := device.Profile{
		Secure:   true,
		Quota:    1000,
		Usage:    200,
		SIMD:     true,
		MemoryGB: 8,
	}
	r := device.Requirement{
		MinFree:     500,
		MinTier:     device.TierSIMD,
		MinRate:     10,
		MinMemoryGB: 4,
	}

	sf := r.Check(p, 20)
	if len(sf) != 0 {
		t.Fatalf("expected empty shortfall, got %v", sf)
	}
}

func TestCheck_InsecureAlwaysReported(t *testing.T) {
	p := device.Profile{Secure: false}
	r := device.Requirement{}

	sf := r.Check(p, 0)
	if len(sf) != 1 || sf[0] != device.ShortSecure {
		t.Fatalf("expected [ShortSecure], got %v", sf)
	}
}

func TestCheck_EachShortfall(t *testing.T) {
	baseP := device.Profile{
		Secure:   true,
		Quota:    1000,
		Usage:    0,
		SIMD:     true,
		MemoryGB: 8,
	}
	baseR := device.Requirement{
		MinFree:     500,
		MinTier:     device.TierSIMD,
		MinRate:     10,
		MinMemoryGB: 4,
	}

	tests := []struct {
		name   string
		p      device.Profile
		r      device.Requirement
		rate   device.Rate
		want   device.Shortfall
	}{
		{
			name: "space",
			p:    device.Profile{Secure: true, Quota: 100, Usage: 50, SIMD: true, MemoryGB: 8},
			r:    baseR,
			rate: 20,
			want: device.ShortSpace,
		},
		{
			name: "tier",
			p:    device.Profile{Secure: true, Quota: 1000, Usage: 0, SIMD: false, MemoryGB: 8},
			r:    baseR,
			rate: 20,
			want: device.ShortTier,
		},
		{
			name: "speed",
			p:    baseP,
			r:    baseR,
			rate: 5,
			want: device.ShortSpeed,
		},
		{
			name: "memory",
			p:    device.Profile{Secure: true, Quota: 1000, Usage: 0, SIMD: true, MemoryGB: 2},
			r:    baseR,
			rate: 20,
			want: device.ShortMemory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := tt.r.Check(tt.p, tt.rate)
			if len(sf) != 1 || sf[0] != tt.want {
				t.Fatalf("expected shortfall [%v], got %v", tt.want, sf)
			}
		})
	}
}

func TestCheck_UnknownMemoryIsNotShort(t *testing.T) {
	p := device.Profile{
		Secure:   true,
		MemoryGB: 0,
	}
	r := device.Requirement{
		MinMemoryGB: 8,
	}

	sf := r.Check(p, 0)
	if len(sf) != 0 {
		t.Fatalf("expected no shortfall for unknown memory 0, got %v", sf)
	}
}

func TestShortfall_OnlyMemoryIsAdvisory(t *testing.T) {
	shortfalls := []device.Shortfall{
		device.ShortSecure,
		device.ShortSpace,
		device.ShortTier,
		device.ShortSpeed,
		device.ShortMemory,
	}

	for _, sf := range shortfalls {
		if sf == device.ShortMemory {
			if sf.Blocking() {
				t.Fatalf("expected ShortMemory.Blocking() == false")
			}
		} else {
			if !sf.Blocking() {
				t.Fatalf("expected %v.Blocking() == true", sf)
			}
		}
	}
}

func TestStrings(t *testing.T) {
	if device.TierNone.String() != "none" ||
		device.TierPlain.String() != "plain" ||
		device.TierSIMD.String() != "simd" ||
		device.TierWebGPU.String() != "webgpu" {
		t.Fatalf("unexpected Tier string values")
	}

	if device.ShortSecure.String() != "secure" ||
		device.ShortSpace.String() != "space" ||
		device.ShortTier.String() != "tier" ||
		device.ShortSpeed.String() != "speed" ||
		device.ShortMemory.String() != "memory" {
		t.Fatalf("unexpected Shortfall string values")
	}
}
