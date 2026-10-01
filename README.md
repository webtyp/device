# `webtyp/device`
<img src="docs/img/badges.svg">

`device` answers one question for a webtyp application running in a browser: **what can this device do?** It detects browser capabilities (storage quota/usage, persistence, logical cores, memory, WebAssembly SIMD, WebGPU, secure context) and measures runtime kernel performance (`Bench`) to evaluate whether the device satisfies an artifact's resource requirements before downloading large model weights.

## Usage Matrix

| I want to... | Use... |
|---|---|
| Query reported browser specs & storage limits | `device.Detect()` |
| Measure execution speed of a WASM/JS kernel | `device.Bench(work, budgetMs)` |
| Verify if a device meets artifact requirements | `req.Check(profile, rate)` |
| Request persistent storage against browser eviction | `device.Persist()` |

## Example

```go
p, err := device.Detect()
if err != nil {
    return err
}

rate := device.Bench(func() { runKernel() }, 200)

req := device.Requirement{
    MinFree: 500 * 1024 * 1024, // 500 MB
    MinTier: device.TierSIMD,
    MinRate: 10,
}

for _, sf := range req.Check(p, rate) {
    if sf.Blocking() {
        log.Printf("Cannot proceed: shortfall %s", sf)
    }
}
```
