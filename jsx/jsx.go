//go:build js

// Package jsx provides essential JavaScript functions that are used
// widely in wgpu and are very useful for an wasm / js application.
package jsx

import (
	"log/slog"
	"syscall/js"
)

// uint8ArrayCtor caches the Uint8Array constructor so BytesToJS does not
// re-resolve it on every call. The constructor object is never detached.
var uint8ArrayCtor = js.Global().Get("Uint8Array")

// BytesToJS copies the given bytes into a freshly allocated, JS-owned
// Uint8Array and returns it.
//
// It deliberately does NOT construct a zero-copy typed-array view over the Go
// wasm linear memory (globalThis.wasm.instance.exports.mem.buffer). Such a view
// aliases the heap's backing ArrayBuffer, which the Go runtime DETACHES whenever
// it grows linear memory (memory.grow). A grow can land at any time a Go
// allocation happens — including inside syscall/js's own makeArgs while
// marshalling the constructor arguments, or in the makeArgs of the outer
// queue.writeBuffer/writeTexture Call that consumes the view — leaving the
// view pointing at a detached buffer and throwing
// "Cannot perform Construct on a detached ArrayBuffer" (trendvidia/fyne#347).
// Reading mem.buffer "fresh" does not help, because the detach happens between
// the fresh read and the construction/consumption.
//
// Copying via js.CopyBytesToJS sidesteps this entirely: the host copy reads
// mem.buffer atomically with no interleaved Go allocation, and the returned
// array is backed by its own JS ArrayBuffer, so a later memory.grow cannot
// detach it. The result is consumed synchronously by writeBuffer/writeTexture,
// so the extra copy is short-lived.
func BytesToJS(b []byte) js.Value {
	dst := uint8ArrayCtor.New(len(b))
	if len(b) > 0 {
		js.CopyBytesToJS(dst, b)
	}
	return dst
}

// Await is a helper function equivalent to await in JS.
// It is copied from https://go-review.googlesource.com/c/go/+/150917/
func Await(promise js.Value) (result js.Value, ok bool) {
	if promise.Type() != js.TypeObject || promise.Get("then").Type() != js.TypeFunction {
		return promise, true
	}

	done := make(chan struct{})

	onResolve := js.FuncOf(func(this js.Value, args []js.Value) any {
		result = args[0]
		ok = true
		close(done)
		return nil
	})
	defer onResolve.Release()

	onReject := js.FuncOf(func(this js.Value, args []js.Value) any {
		result = args[0]
		ok = false
		slog.Error("wgpu.AwaitJS: promise rejected", "reason", result)
		close(done)
		return nil
	})
	defer onReject.Release()

	promise.Call("then", onResolve, onReject)
	<-done
	return
}
