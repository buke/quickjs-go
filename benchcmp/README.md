# benchcmp

`benchcmp` is a standalone Go submodule in this repository. From the repo root, run it with `cd benchcmp && go run .`.

It runs a README-style comparison across:

- [quickjs-go（cgo / QuickJS-ng）](https://github.com/buke/quickjs-go): current repository binding, based on native QuickJS/quickjs-ng via cgo.
- [GOJA（pure Go）](https://github.com/dop251/goja): pure Go JavaScript engine, easy to embed and portable.
- [ModerncQuickJS（ccgo / QuickJS）](https://pkg.go.dev/modernc.org/quickjs): QuickJS translated to Go via ccgo, so it avoids cgo but is not a hand-written Go engine.
- [QJS（Wasm / wazero）](https://github.com/fastschema/qjs): runs QuickJS through WebAssembly on top of wazero.
- [V8go（cgo / V8 JIT）](https://github.com/tommie/v8go): Go binding for V8 with JIT-backed execution.

The first engine column is used as the `1.00x` baseline for the relative rows. By default, that means quickjs-go is the baseline because it is listed first.

Default suites:

- `factorial`: computes `factorial(10)` 1,000,000 times for 5 measured runs
- `v8-v7`: runs the V8 benchmark suite version 7 workloads used by the qjs README naming scheme

Examples:

```bash
cd benchcmp && go run .
cd benchcmp && go run . -suites factorial
cd benchcmp && go run . -engines quickjs-go,goja,qjs,v8go
```

The `v8-v7` JavaScript assets are vendored under `assets/v8-v7` inside the submodule, so the command does not depend on the caller's module cache. Their provenance is preserved through the copied benchmark files and license material in the same assets directory.