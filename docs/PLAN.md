---
PLAN: "feat: Profile, Tier, Bench and Requirement — what this browser can do"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 8681699153495066669
PR: https://github.com/webtyp/device/pull/1
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `webtyp/device` v0.1.0

Master plan: [PWA_ARTIFACTS_MASTER_PLAN.md](https://github.com/webtyp/app/blob/main/docs/PWA_ARTIFACTS_MASTER_PLAN.md),
decisions D-PWA-1, D-PWA-2, D-PWA-3, D-PWA-9. **Read [AGENTS.md](../AGENTS.md) first**: forbidden
imports, the TinyGo build that decides, test layout.

## Why

A webtyp application can download 400–850 MB of model weights into the browser. Before doing that
it must know whether the device can hold and run them. Memory and speed cannot be *asked* to a
browser, only measured; space, cores, SIMD and WebGPU can be asked. This repo is fresh (`gonew`):
`device.go` holds a placeholder `Device` type and `New()` — **delete both**.

## Design gate

1. **Prior art.**
   - **`wasm-feature-detect`** (Google Chrome Labs) validates tiny hand-made modules with
     `WebAssembly.validate` to detect SIMD, threads, etc. We use its SIMD module bytes.
   - **WebLLM** checks `navigator.gpu.requestAdapter()` and the adapter's limits before loading a
     model and refuses with a message when it is missing — it only asks, never measures speed.
   - **TensorFlow.js** picks a backend (`webgpu` → `wasm` with SIMD → `wasm` → `cpu`) by feature test,
     in a fixed priority list — the same three tiers as decision D25 of the agent wave.
   - What differs here: none of them **measure**; D-PWA-3 says a benchmark decides, because
     `deviceMemory` does not exist in Firefox/Safari and a slow CPU with SIMD is still slow. So the
     API separates *asking* (`Detect`) from *measuring* (`Bench`) and *deciding* (`Requirement.Check`).
2. **Novice-name test.** `device.Detect()` → "detect the device". `profile.Tier()`,
   `device.Bench(work, 200)` → "benchmark this work for 200 ms". `req.Check(profile, rate)` →
   "check the requirement against the profile and rate", returns the `Shortfall`s: what is short.
   `device.Persist()` → "ask the browser to persist storage".
3. **Complexity ledger.** New library: +5 concepts (`Profile`, `Tier`, `Rate`, `Requirement`,
   `Shortfall`), 3 functions. Ways to do the same thing: 0 — nobody detects devices today
   (`weights.QuotaEstimator` is deleted by another plan of the same wave).
4. **Where it belongs.** Its own repo (D-PWA-1): the agent Worker, the application and the UI all
   ask it; `js` stays the general browser API. No dependency on `nn`, `files` or `opfs`.
5. **What it deletes.** The `gonew` placeholder. Otherwise new capability.

## Stage 1 — `device.go` (no build tag): the pure types

Write exactly this API (doc comments in English, as shown):

```go
package device

// Tier is how fast this browser can run WebAssembly kernels, best last.
type Tier uint8

const (
	TierNone   Tier = iota // not detected
	TierPlain              // WebAssembly without SIMD
	TierSIMD               // WebAssembly with SIMD128
	TierWebGPU             // a WebGPU adapter is available
)

// String returns "none", "plain", "simd" or "webgpu".
func (t Tier) String() string

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
func (p Profile) Free() int64

// Tier returns TierWebGPU when WebGPU, else TierSIMD when SIMD, else TierPlain.
func (p Profile) Tier() Tier

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

// Blocking reports whether the shortfall forbids the download. Only ShortMemory does not.
func (s Shortfall) Blocking() bool

// String returns "secure", "space", "tier", "speed" or "memory".
func (s Shortfall) String() string

// Check lists what p and the measured rate lack for r, in the order of the constants above.
// An empty result means the requirement is met.
func (r Requirement) Check(p Profile, rate Rate) []Shortfall
```

Rules for `Check` (each a line of code, no other conditions):
- `ShortSecure` when `!p.Secure` — always, whatever `r` says.
- `ShortSpace` when `r.MinFree > 0 && p.Free() < r.MinFree`.
- `ShortTier` when `r.MinTier > TierNone && p.Tier() < r.MinTier`.
- `ShortSpeed` when `r.MinRate > 0 && rate < r.MinRate`.
- `ShortMemory` when `r.MinMemoryGB > 0 && p.MemoryGB > 0 && p.MemoryGB < r.MinMemoryGB`
  (an unknown memory, `0`, never produces a shortfall).

String values are unexported named constants.

## Stage 2 — `detect.go` (`//go:build wasm`): ask and measure

```go
// Detect asks the browser what this device can do. It blocks on browser promises: call it from a
// goroutine. It works in a page and in a Worker (it reads the global scope, not window).
func Detect() (Profile, error)

// Persist asks the browser not to evict this origin's storage and reports whether it is now
// persistent. Call it right after a user gesture (e.g. the click that starts the first large
// download), never at page load: Firefox shows a prompt and Chrome refuses without engagement.
func Persist() (bool, error)

// Bench runs work once to warm up, then repeatedly for at least budgetMs milliseconds (and at
// least 3 times), and returns how many times per second it ran. Time is read with
// performance.now().
func Bench(work func(), budgetMs int) Rate
```

Implementation, reading `g := js.Global()`:
- `Secure` = `g.Get("isSecureContext").Bool()` (false when undefined).
- `nav := g.Get("navigator")`. `Cores` = `hardwareConcurrency` (0 when undefined).
  `MemoryGB` = `deviceMemory` as float (0 when undefined).
- Storage: `st := nav.Get("storage")`. When `st` is undefined (insecure context) leave `Quota`,
  `Usage`, `Persisted` at zero and **return no error**: `Secure` already explains it. Otherwise
  `await.Promise(st.Call("estimate"))` → `quota`, `usage` (as `int64` from `Float()`); and
  `await.Promise(st.Call("persisted"))` → `Persisted`. A rejected promise is returned as an error
  `device: <message>` (constant `errBrowser = "device: %v"` used with `fmt.Errf`).
- SIMD: `g.Get("WebAssembly").Call("validate", u8)` where `u8` is a `Uint8Array` built with
  `js.CopyBytesToJS` from this package-level constant (from `wasm-feature-detect`, a module whose
  only function uses `v128` instructions):
  ```go
  var simdProbe = []byte{0, 97, 115, 109, 1, 0, 0, 0, 1, 5, 1, 96, 0, 1, 123, 3, 2, 1, 0, 10, 10, 1, 8, 0, 65, 0, 253, 15, 253, 98, 11}
  ```
- WebGPU: when `nav.Get("gpu")` is undefined → false; else `await.Promise(gpu.Call("requestAdapter"))`
  → `true` when the result is neither null nor undefined. A rejected promise here means **false**,
  not an error (headless browsers reject).
- `Persist`: `st` undefined → `false, nil`; else `await.Promise(st.Call("persist"))` → bool.
- `Bench`: `now := g.Get("performance")`; call `work()` once untimed; then loop calling `work()`
  and counting until `elapsed >= budgetMs` **and** `runs >= 3`; return
  `Rate(float64(runs) / (elapsedMs / 1000))`. `budgetMs <= 0` is treated as `1`.

## Stage 3 — tests (`tests/`, `package tests`)

`tests/device_test.go` (no build tag):

| Test | Proves |
|---|---|
| `TestProfile_Free_NeverNegative` | `Usage > Quota` → 0 |
| `TestProfile_Tier` | table: none/SIMD/WebGPU/both → Plain/SIMD/WebGPU/WebGPU |
| `TestCheck_MetIsEmpty` | a secure profile with room, SIMD, rate above → `len == 0` |
| `TestCheck_InsecureAlwaysReported` | `Secure: false` with zero `Requirement` → `[ShortSecure]` |
| `TestCheck_EachShortfall` | table, one field short at a time → exactly that shortfall |
| `TestCheck_UnknownMemoryIsNotShort` | `MemoryGB: 0`, `MinMemoryGB: 8` → no `ShortMemory` |
| `TestShortfall_OnlyMemoryIsAdvisory` | `Blocking()` false only for `ShortMemory` |
| `TestStrings` | `Tier.String` and `Shortfall.String` values above |

`tests/detect_test.go` (`//go:build wasm`, runs in headless Chrome through `gotest`):

| Test | Proves |
|---|---|
| `TestDetect_InBrowser` | no error; `Cores > 0`; `SIMD == true` (Chrome supports SIMD128 — if this fails, the probe bytes are wrong, do not weaken the test) |
| `TestPersist_NoError` | returns without error (the value may be false headless) |
| `TestBench_CountsRuns` | `Bench(func(){ n++ }, 20)` → `n >= 4` (warm-up + 3) and `Rate > 0` |
| `TestBench_ConsumerShaped` | a consumer flow: `p, _ := Detect()`; `r := Bench(kernel, 50)` where `kernel` multiplies two 64×64 float32 slices; `Requirement{MinTier: TierPlain, MinRate: 1}.Check(p, r)` → no **blocking** shortfall other than `ShortSecure`/`ShortSpace` (the test page may be served over plain HTTP: assert only on the others) |

## Stage 4 — docs

- `README.md`: one paragraph of what it is, then an "I want X → use Y" table (detect, measure,
  decide, ask persistence) and one 10-line example: `Detect` → `Bench` → `Check` → loop over
  shortfalls with `Blocking()`.
- `docs/ARCHITECTURE.md`: the "what the browser lets you know" table from the master plan (API and
  limit per question), why `Bench` takes a function (no dependency on `nn`), why memory is advisory
  (D-PWA-3), when to call `Persist` (D-PWA-9), and why `Secure` is part of the profile (D-PWA-13:
  without HTTPS there is no OPFS).

## Acceptance

- `gotest` and `gotest -tinygo` green.
- `grep -rn "type Device\|func New()" --include=*.go .` → empty.
- `grep -rln "\"fmt\"\|\"errors\"\|\"strings\"\|\"strconv\"\|\"encoding/json\"\|\"time\"" --include=*.go . | grep -v _test` → empty.

| Stage | Files | Done when |
|---|---|---|
| 1 | `device.go` | pure types, placeholder deleted |
| 2 | `detect.go` | browser detection, `Persist`, `Bench` |
| 3 | `tests/device_test.go`, `tests/detect_test.go` | tables green in Go and in the browser |
| 4 | `README.md`, `docs/ARCHITECTURE.md` | written |
