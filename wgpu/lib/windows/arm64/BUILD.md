# windows/arm64 `libwgpu_native.a` build recipe

Unlike the other platforms, upstream **gfx-rs/wgpu-native ships only an
`aarch64-pc-windows-msvc`** build for Windows-on-ARM. That MSVC-ABI archive
(MSVC name mangling, SEH `__CxxFrameHandler` C++ exception handling, `vcruntime`
deps) **cannot be linked by the GNU/llvm-mingw toolchain** that Go/cgo uses on
windows/arm64 — the symbols (`??_7type_info@@6B@`, `__CxxFrameHandler`,
`_purecall`, …) have no mingw provider and the C++ EH models are incompatible.
(See trendvidia/fyne#609.)

So this archive is built **from source with the GNU ABI** (`aarch64-pc-windows-gnullvm`),
matching how `windows/amd64` uses `x86_64-pc-windows-gnu`. Verified: 0 MSVC ABI
symbols, links with llvm-mingw + `-lole32 -loleaut32` (added to the windows
`LDFLAGS` in `wgpu/wgpu.go`), and creates a working D3D12 instance/adapter.

Built natively on a Windows-arm64 host (2026-07-06), wgpu-native **v22.1.0.5**
(matches the vendored `wgpu/lib/webgpu.h`):

```
# Rust with the GNU (gnullvm) host toolchain — no MSVC needed; links via llvm-mingw
rustup-init --default-host aarch64-pc-windows-gnullvm --default-toolchain stable --profile minimal -y

git clone --recursive --branch v22.1.0.5 --depth 1 https://github.com/gfx-rs/wgpu-native.git

# wgpu-native's build.rs runs bindgen -> needs an aarch64 libclang.dll:
#   clang+llvm-<ver>-aarch64-pc-windows-msvc.tar.xz from llvm/llvm-project releases
# Bump bindgen so it understands a modern libclang AST (0.70 mis-traverses -> opaque structs):
#   Cargo.toml:  bindgen = "0.72"   (build-only dep; does not change the FFI ABI)

set LIBCLANG_PATH=<llvm>\bin
# GNU-mode clang parse (mingw vadefs.h only has an MSVC-macro ARM64 branch):
set BINDGEN_EXTRA_CLANG_ARGS_aarch64_pc_windows_gnullvm=--target=aarch64-w64-mingw32 --sysroot=<llvm-mingw>/aarch64-w64-mingw32

cargo build --release            # host == target == aarch64-pc-windows-gnullvm
# -> target/release/libwgpu_native.a
```

TODO (trendvidia/fyne#609 follow-up): fold an `aarch64-pc-windows-gnullvm` entry
into `.github/workflows/build-wgpu.yml` (pinned + libclang/bindgen setup) so a
full-set regen produces this automatically instead of by hand.
