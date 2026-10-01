//go:build wasm

package device

import (
	"syscall/js"

	"webtyp.com/await"
	"webtyp.com/fmt"
)

const errBrowser = "device: %v"

var simdProbe = []byte{0, 97, 115, 109, 1, 0, 0, 0, 1, 5, 1, 96, 0, 1, 123, 3, 2, 1, 0, 10, 10, 1, 8, 0, 65, 0, 253, 15, 253, 98, 11}

// Detect asks the browser what this device can do. It blocks on browser promises: call it from a
// goroutine. It works in a page and in a Worker (it reads the global scope, not window).
func Detect() (Profile, error) {
	g := js.Global()
	var p Profile

	if isSec := g.Get("isSecureContext"); isSec.Type() == js.TypeBoolean {
		p.Secure = isSec.Bool()
	}

	nav := g.Get("navigator")
	if nav.Type() != js.TypeUndefined {
		if cores := nav.Get("hardwareConcurrency"); cores.Type() == js.TypeNumber {
			p.Cores = cores.Int()
		}
		if mem := nav.Get("deviceMemory"); mem.Type() == js.TypeNumber {
			p.MemoryGB = mem.Float()
		}

		st := nav.Get("storage")
		if st.Type() != js.TypeUndefined {
			estPromise := st.Call("estimate")
			estVal, err := await.Promise(estPromise)
			if err != nil {
				return p, fmt.Errf(errBrowser, err)
			}
			p.Quota = int64(estVal.Get("quota").Float())
			p.Usage = int64(estVal.Get("usage").Float())

			persPromise := st.Call("persisted")
			persVal, err := await.Promise(persPromise)
			if err != nil {
				return p, fmt.Errf(errBrowser, err)
			}
			p.Persisted = persVal.Bool()
		}

		gpu := nav.Get("gpu")
		if gpu.Type() != js.TypeUndefined {
			adapterPromise := gpu.Call("requestAdapter")
			adapterVal, err := await.Promise(adapterPromise)
			if err == nil && adapterVal.Type() != js.TypeNull && adapterVal.Type() != js.TypeUndefined {
				p.WebGPU = true
			}
		}
	}

	wasm := g.Get("WebAssembly")
	if wasm.Type() != js.TypeUndefined {
		u8 := g.Get("Uint8Array").New(len(simdProbe))
		js.CopyBytesToJS(u8, simdProbe)
		valid := wasm.Call("validate", u8)
		if valid.Type() == js.TypeBoolean {
			p.SIMD = valid.Bool()
		}
	}

	return p, nil
}

// Persist asks the browser not to evict this origin's storage and reports whether it is now
// persistent. Call it right after a user gesture (e.g. the click that starts the first large
// download), never at page load: Firefox shows a prompt and Chrome refuses without engagement.
func Persist() (bool, error) {
	g := js.Global()
	nav := g.Get("navigator")
	if nav.Type() == js.TypeUndefined {
		return false, nil
	}
	st := nav.Get("storage")
	if st.Type() == js.TypeUndefined {
		return false, nil
	}

	persPromise := st.Call("persist")
	persVal, err := await.Promise(persPromise)
	if err != nil {
		return false, fmt.Errf(errBrowser, err)
	}
	return persVal.Bool(), nil
}

// Bench runs work once to warm up, then repeatedly for at least budgetMs milliseconds (and at
// least 3 times), and returns how many times per second it ran. Time is read with
// performance.now().
func Bench(work func(), budgetMs int) Rate {
	if budgetMs <= 0 {
		budgetMs = 1
	}

	g := js.Global()
	perf := g.Get("performance")

	work() // warm up untimed

	startMs := perf.Call("now").Float()
	runs := 0
	var elapsedMs float64

	for {
		work()
		runs++
		nowMs := perf.Call("now").Float()
		elapsedMs = nowMs - startMs
		if elapsedMs >= float64(budgetMs) && runs >= 3 {
			break
		}
	}

	if elapsedMs <= 0 {
		return 0
	}
	return Rate(float64(runs) / (elapsedMs / 1000.0))
}
