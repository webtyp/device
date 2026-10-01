# Architecture & Design Decisions

`device` provides device detection, benchmarking, and requirement checks for browser-based webtyp applications.

## What the Browser Lets You Know

| Question | Browser API | Limit / Behavior |
|---|---|---|
| Is page secure? | `isSecureContext` | `false` on plain HTTP; required for OPFS and Service Workers |
| How much storage is available? | `navigator.storage.estimate()` | Returns origin quota and usage bytes |
| Is storage persistent? | `navigator.storage.persisted()` | `true` if origin storage won't be evicted under pressure |
| How many CPU cores? | `navigator.hardwareConcurrency` | Returns logical core count; missing/undefined returns `0` |
| How much RAM? | `navigator.deviceMemory` | Approximate RAM in GB; missing in Firefox/Safari (returns `0`) |
| Is SIMD supported? | `WebAssembly.validate()` | Validates a WASM probe containing `v128` instructions |
| Is WebGPU available? | `navigator.gpu.requestAdapter()` | `true` when requestAdapter resolves to a non-null adapter |

## Key Design Decisions

### 1. `Bench` Accepts a Generic Kernel Function
`Bench` accepts an arbitrary `work func()` callback rather than taking dependencies on neural net (`nn`) or matrix libraries. This keeps `device` decoupled from model execution libraries while allowing callers to measure their specific runtime kernel or operations.

### 2. Memory is Advisory (`ShortMemory`)
As established in decision **D-PWA-3**, `navigator.deviceMemory` is non-standard and omitted by browsers like Firefox and Safari to protect user privacy. Because unknown memory reports as `0`, a missing memory value never triggers a shortfall. When reported, memory shortfall (`ShortMemory`) is purely advisory (`Blocking() == false`). Speed and tier benchmarks (`Bench`, `Tier`) are the primary deciders for performance suitability.

### 3. Storage Persistence Timing (`Persist`)
Decision **D-PWA-9** dictates that `Persist()` should be called immediately following an explicit user gesture (such as clicking a button to start a download). Calling `Persist()` on initial page load causes Firefox to display intrusive prompts and Chrome to reject the request due to lack of user engagement.

### 4. Secure Context (`Secure`)
Decision **D-PWA-13** highlights that modern storage APIs like OPFS (Origin Private File System) and Service Workers strictly require a secure context (HTTPS or `localhost`). `Profile.Secure` captures `isSecureContext`; if `!p.Secure`, `Check()` always reports `ShortSecure`, as heavy model downloads cannot be stored or operated safely without secure context capabilities.
