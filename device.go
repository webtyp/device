package device

// Tier is how fast this browser can run WebAssembly kernels, best last.
type Tier uint8

const (
	TierNone  Tier = iota // not detected
	TierPlain             // WebAssembly without SIMD
	TierSIMD              // WebAssembly with SIMD128
	TierWebGPU            // a WebGPU adapter is available
)

const (
	strNone   = "none"
	strPlain  = "plain"
	strSIMD   = "simd"
	strWebGPU = "webgpu"
)

// String returns "none", "plain", "simd" or "webgpu".
func (t Tier) String() string {
	switch t {
	case TierPlain:
		return strPlain
	case TierSIMD:
		return strSIMD
	case TierWebGPU:
		return strWebGPU
	default:
		return strNone
	}
}

// Profile is what the browser reports about this device. Zero fields mean the browser did not say.
type Profile struct {
	Secure    bool    // the page is a secure context (HTTPS or localhost); without it there is no OPFS nor service worker
	Quota     int64   // bytes this origin may use (navigator.storage.estimate)
	Usage     int64   // bytes this origin already uses
	Persisted bool    // the browser will not evict this origin's storage
	MemoryGB  float64 // navigator.deviceMemory; 0 when the browser does not expose it
	Cores     int     // navigator.hardwareConcurrency (logical)
	SIMD      bool
	WebGPU    bool
}

// Free returns Quota minus Usage, never negative.
func (p Profile) Free() int64 {
	free := p.Quota - p.Usage
	if free < 0 {
		return 0
	}
	return free
}

// Tier returns TierWebGPU when WebGPU, else TierSIMD when SIMD, else TierPlain.
func (p Profile) Tier() Tier {
	if p.WebGPU {
		return TierWebGPU
	}
	if p.SIMD {
		return TierSIMD
	}
	return TierPlain
}

// Rate is how many times per second a benchmarked function ran.
type Rate float64

// Requirement is what one artifact needs from the device. Zero fields require nothing.
type Requirement struct {
	MinFree     int64   // bytes that must be free before downloading
	MinTier     Tier
	MinRate     Rate    // runs per second of the caller's kernel, measured with Bench
	MinMemoryGB float64 // advisory: only a warning, the browser may not report memory
}

// Shortfall is one thing the device lacks for a Requirement.
type Shortfall uint8

const (
	ShortSecure Shortfall = iota + 1 // the page is not served over HTTPS
	ShortSpace
	ShortTier
	ShortSpeed
	ShortMemory // advisory
)

const (
	strSecure = "secure"
	strSpace  = "space"
	strTier   = "tier"
	strSpeed  = "speed"
	strMemory = "memory"
)

// Blocking reports whether the shortfall forbids the download. Only ShortMemory does not.
func (s Shortfall) Blocking() bool {
	return s != ShortMemory
}

// String returns "secure", "space", "tier", "speed" or "memory".
func (s Shortfall) String() string {
	switch s {
	case ShortSecure:
		return strSecure
	case ShortSpace:
		return strSpace
	case ShortTier:
		return strTier
	case ShortSpeed:
		return strSpeed
	case ShortMemory:
		return strMemory
	default:
		return ""
	}
}

// Check lists what p and the measured rate lack for r, in the order of the constants above.
// An empty result means the requirement is met.
func (r Requirement) Check(p Profile, rate Rate) []Shortfall {
	var shortfalls []Shortfall
	if !p.Secure {
		shortfalls = append(shortfalls, ShortSecure)
	}
	if r.MinFree > 0 && p.Free() < r.MinFree {
		shortfalls = append(shortfalls, ShortSpace)
	}
	if r.MinTier > TierNone && p.Tier() < r.MinTier {
		shortfalls = append(shortfalls, ShortTier)
	}
	if r.MinRate > 0 && rate < r.MinRate {
		shortfalls = append(shortfalls, ShortSpeed)
	}
	if r.MinMemoryGB > 0 && p.MemoryGB > 0 && p.MemoryGB < r.MinMemoryGB {
		shortfalls = append(shortfalls, ShortMemory)
	}
	return shortfalls
}
