# AGENTS.md — webtyp/device

Working notes for AI agents operating in this library. End-user docs: [README.md](README.md).

## Mission

`device` answers one question for a webtyp application running in a browser: **what can this
device do?** Storage quota and usage, whether storage is persistent, memory (when the browser says),
cores, WebAssembly SIMD, WebGPU, whether the page is a secure context, and a **measured** speed
(`Bench`). Callers compare that against a `Requirement` before downloading hundreds of MB.

It does not download, store or run models. It does not import `nn` or any model package:
`Bench` measures whatever function the caller passes.

## The build that decides

This is a **browser library**. It is compiled by TinyGo to WebAssembly in production. A change is
done only when all of these pass:

```bash
go install webtyp.com/devflow/cmd/gotest@latest   # once; needs gotest >= v0.4.108
gotest            # vet + tests + the wasm tests in tests/ run in a headless browser
gotest -tinygo    # also compiles with TinyGo — mandatory
```

`GOOS=js GOARCH=wasm go build` succeeding proves nothing: it uses the **full** standard library and
does not imply TinyGo. `net/http` compiles there and is still forbidden.

## Forbidden imports and their replacement

| Do not import | Use instead |
|---|---|
| `fmt`, `errors`, `strconv`, `strings` | `webtyp.com/fmt` |
| `encoding/json` (adds ~1 MB of wasm under TinyGo) | `webtyp.com/json` |
| `net/http` | `webtyp.com/fetch` |
| `context` | `webtyp.com/context` |
| `time` | `webtyp.com/time`, or `performance.now()` through `syscall/js` |
| `reflect` | nothing — plain structs |
| `map[K]V` (heavy in TinyGo) | a slice of structs scanned linearly, or `fmt.KeyValue` |

Waiting on a JavaScript promise: `webtyp.com/await` (`await.Promise(p)`), never a hand-written
channel + callback pair. Do not invent a port or helper that another `webtyp.com/*` package
already exposes.

## Layout

| File | Build | Role |
|---|---|---|
| `device.go` | all | `Tier`, `Profile`, `Rate`, `Requirement`, `Shortfall` — pure Go, testable anywhere |
| `detect.go` | `wasm` | `Detect`, `Persist`, `Bench` — read the browser through `syscall/js` |
| `tests/` | — | `device_test.go` (all builds), `detect_test.go` (`//go:build wasm`, runs in the browser) |

All tests live in `tests/` as `package tests`, using only the public API.
