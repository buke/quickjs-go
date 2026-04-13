package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	qjswasm "github.com/fastschema/qjs"
	v8 "github.com/tommie/v8go"
	moderncquickjs "modernc.org/quickjs"

	quickjs "github.com/buke/quickjs-go"
)

type engine struct {
	id         string
	display    string
	evalInt    func(string) (int64, error)
	evalString func(string) (string, error)
	runV8V7    func() (string, error)
}

func supportedEngines() []engine {
	return []engine{
		{
			id:      "goja",
			display: "GOJA（pure Go）",
			evalInt: func(script string) (int64, error) {
				vm := goja.New()
				value, err := vm.RunString(script)
				if err != nil {
					return 0, err
				}
				return normalizeInt(value.Export())
			},
			evalString: func(script string) (string, error) {
				vm := goja.New()
				value, err := vm.RunString(script)
				if err != nil {
					return "", err
				}
				return normalizeString(value.Export())
			},
		},
		{
			id:      "moderncquickjs",
			display: "ModerncQuickJS（ccgo / QuickJS）",
			evalInt: func(script string) (int64, error) {
				vm, err := moderncquickjs.NewVM()
				if err != nil {
					return 0, err
				}
				defer vm.Close()

				value, err := vm.Eval(script, moderncquickjs.EvalGlobal)
				if err != nil {
					return 0, err
				}
				return normalizeInt(value)
			},
			evalString: func(script string) (string, error) {
				vm, err := moderncquickjs.NewVM()
				if err != nil {
					return "", err
				}
				defer vm.Close()

				value, err := vm.Eval(script, moderncquickjs.EvalGlobal)
				if err != nil {
					return "", err
				}
				return normalizeString(value)
			},
		},
		{
			id:      "qjs",
			display: "QJS（Wasm / wazero）",
			evalInt: func(script string) (int64, error) {
				runtime, err := qjswasm.New()
				if err != nil {
					return 0, err
				}
				defer runtime.Close()

				value, err := runtime.Context().Eval("factorial.js", qjswasm.Code(wrapQJSScript(script)))
				if err != nil {
					return 0, err
				}
				defer value.Free()
				return value.Int64(), nil
			},
			evalString: func(script string) (string, error) {
				runtime, err := qjswasm.New()
				if err != nil {
					return "", err
				}
				defer runtime.Close()

				value, err := runtime.Context().Eval("v8-v7.js", qjswasm.Code(wrapQJSScript(script)))
				if err != nil {
					return "", err
				}
				defer value.Free()
				return value.String(), nil
			},
			runV8V7: func() (string, error) {
				runtime, err := qjswasm.New()
				if err != nil {
					return "", err
				}
				defer runtime.Close()

				ctx := runtime.Context()
				scripts, err := v8V7AssetScripts()
				if err != nil {
					return "", err
				}

				allScripts := append([]string{v8V7PreludeScript()}, scripts...)
				for index, script := range allScripts {
					value, err := ctx.Eval(fmt.Sprintf("v8-v7-%d.js", index), qjswasm.Code(wrapQJSScript(script)))
					if err != nil {
						return "", err
					}
					if value != nil {
						value.Free()
					}
				}

				value, err := ctx.Eval("v8-v7-runner.js", qjswasm.Code(wrapQJSScript(v8V7RunnerScript())))
				if err != nil {
					return "", err
				}
				defer value.Free()
				return value.String(), nil
			},
		},
		{
			id:      "v8go",
			display: "V8go（cgo / V8 JIT）",
			evalInt: func(script string) (int64, error) {
				iso := v8.NewIsolate()
				defer iso.Dispose()

				ctx := v8.NewContext(iso)
				defer ctx.Close()

				value, err := ctx.RunScript(script, "factorial.js")
				if err != nil {
					return 0, err
				}
				if value == nil {
					return 0, fmt.Errorf("v8go eval returned nil")
				}
				return normalizeInt(value.String())
			},
			evalString: func(script string) (string, error) {
				iso := v8.NewIsolate()
				defer iso.Dispose()

				ctx := v8.NewContext(iso)
				defer ctx.Close()

				value, err := ctx.RunScript(script, "v8-v7.js")
				if err != nil {
					return "", err
				}
				if value == nil {
					return "", fmt.Errorf("v8go eval returned nil")
				}
				return value.String(), nil
			},
		},
		{
			id:      "quickjs-go",
			display: "quickjs-go（cgo / QuickJS-ng）",
			evalInt: func(script string) (int64, error) {
				runtime := quickjs.NewRuntime()
				defer runtime.Close()

				ctx := runtime.NewContext()
				defer ctx.Close()

				value := ctx.Eval(script)
				if value == nil {
					return 0, fmt.Errorf("quickjs-go eval returned nil")
				}
				defer value.Free()
				if value.IsException() || ctx.HasException() {
					return 0, ctx.Exception()
				}
				return value.Int64(), nil
			},
			evalString: func(script string) (string, error) {
				runtime := quickjs.NewRuntime()
				defer runtime.Close()

				ctx := runtime.NewContext()
				defer ctx.Close()

				value := ctx.Eval(script)
				if value == nil {
					return "", fmt.Errorf("quickjs-go eval returned nil")
				}
				defer value.Free()
				if value.IsException() || ctx.HasException() {
					return "", ctx.Exception()
				}
				return value.String(), nil
			},
		},
	}
}

func selectEngines(names []string) ([]engine, error) {
	all := supportedEngines()
	index := make(map[string]engine, len(all))
	for _, item := range all {
		index[item.id] = item
	}

	result := make([]engine, 0, len(names))
	for _, name := range names {
		normalized := normalizeEngineName(name)
		item, ok := index[normalized]
		if !ok {
			return nil, fmt.Errorf("unsupported engine %q", name)
		}
		result = append(result, item)
	}
	return result, nil
}

func normalizeEngineName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "modernc", "modernc-quickjs", "modernc_quickjs":
		return "moderncquickjs"
	case "v8", "v8-go", "tommie/v8go":
		return "v8go"
	case "quickjsgo", "quickjs_go":
		return "quickjs-go"
	default:
		return name
	}
}

func normalizeInt(value any) (int64, error) {
	switch current := value.(type) {
	case int:
		return int64(current), nil
	case int32:
		return int64(current), nil
	case int64:
		return current, nil
	case float64:
		return int64(current), nil
	case string:
		parsed, err := strconv.ParseInt(current, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse int %q: %w", current, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported integer result type %T", value)
	}
}

func wrapQJSScript(script string) string {
	return "(0, eval)(" + strconv.Quote(script) + ")"
}

func normalizeString(value any) (string, error) {
	switch current := value.(type) {
	case string:
		return current, nil
	case []byte:
		return string(current), nil
	default:
		return "", fmt.Errorf("unsupported string result type %T", value)
	}
}
