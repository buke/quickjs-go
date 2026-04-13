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
	id      string
	display string
	open    func() (engineSession, error)
}

type engineSession interface {
	evalInt(script string) (int64, error)
	evalString(script string) (string, error)
	close()
}

type engineV8V7Runner interface {
	runV8V7() (string, error)
}

type gojaSession struct {
	vm *goja.Runtime
}

func newGojaSession() (engineSession, error) {
	return &gojaSession{vm: goja.New()}, nil
}

func (s *gojaSession) evalInt(script string) (int64, error) {
	value, err := s.vm.RunString(script)
	if err != nil {
		return 0, err
	}
	return normalizeInt(value.Export())
}

func (s *gojaSession) evalString(script string) (string, error) {
	value, err := s.vm.RunString(script)
	if err != nil {
		return "", err
	}
	return normalizeString(value.Export())
}

func (s *gojaSession) close() {}

type moderncQuickJSSession struct {
	vm *moderncquickjs.VM
}

func newModerncQuickJSSession() (engineSession, error) {
	vm, err := moderncquickjs.NewVM()
	if err != nil {
		return nil, err
	}
	return &moderncQuickJSSession{vm: vm}, nil
}

func (s *moderncQuickJSSession) evalInt(script string) (int64, error) {
	value, err := s.vm.Eval(script, moderncquickjs.EvalGlobal)
	if err != nil {
		return 0, err
	}
	return normalizeInt(value)
}

func (s *moderncQuickJSSession) evalString(script string) (string, error) {
	value, err := s.vm.Eval(script, moderncquickjs.EvalGlobal)
	if err != nil {
		return "", err
	}
	return normalizeString(value)
}

func (s *moderncQuickJSSession) close() {
	if s.vm != nil {
		s.vm.Close()
	}
}

type qjsSession struct {
	runtime *qjswasm.Runtime
	ctx     *qjswasm.Context
}

func newQJSSession() (engineSession, error) {
	runtime, err := qjswasm.New()
	if err != nil {
		return nil, err
	}
	return &qjsSession{
		runtime: runtime,
		ctx:     runtime.Context(),
	}, nil
}

func (s *qjsSession) evalInt(script string) (int64, error) {
	value, err := s.ctx.Eval("factorial.js", qjswasm.Code(wrapQJSScript(script)))
	if err != nil {
		return 0, err
	}
	defer value.Free()
	return value.Int64(), nil
}

func (s *qjsSession) evalString(script string) (string, error) {
	value, err := s.ctx.Eval("v8-v7.js", qjswasm.Code(wrapQJSScript(script)))
	if err != nil {
		return "", err
	}
	defer value.Free()
	return value.String(), nil
}

func (s *qjsSession) runV8V7() (string, error) {
	scripts, err := v8V7AssetScripts()
	if err != nil {
		return "", err
	}

	allScripts := append([]string{v8V7PreludeScript()}, scripts...)
	for index, script := range allScripts {
		value, err := s.ctx.Eval(fmt.Sprintf("v8-v7-%d.js", index), qjswasm.Code(wrapQJSCompatScript(script)))
		if err != nil {
			return "", err
		}
		if value != nil {
			value.Free()
		}
	}

	value, err := s.ctx.Eval("v8-v7-runner.js", qjswasm.Code(wrapQJSCompatScript(v8V7RunnerScript())))
	if err != nil {
		return "", err
	}
	defer value.Free()
	return value.String(), nil
}

func (s *qjsSession) close() {
	if s.runtime != nil {
		s.runtime.Close()
	}
}

type v8goSession struct {
	iso *v8.Isolate
	ctx *v8.Context
}

func newV8goSession() (engineSession, error) {
	iso := v8.NewIsolate()
	ctx := v8.NewContext(iso)
	return &v8goSession{iso: iso, ctx: ctx}, nil
}

func (s *v8goSession) evalInt(script string) (int64, error) {
	value, err := s.ctx.RunScript(script, "factorial.js")
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, fmt.Errorf("v8go eval returned nil")
	}
	return value.Integer(), nil
}

func (s *v8goSession) evalString(script string) (string, error) {
	value, err := s.ctx.RunScript(script, "v8-v7.js")
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", fmt.Errorf("v8go eval returned nil")
	}
	return value.String(), nil
}

func (s *v8goSession) close() {
	if s.ctx != nil {
		s.ctx.Close()
	}
	if s.iso != nil {
		s.iso.Dispose()
	}
}

type quickjsGoSession struct {
	runtime *quickjs.Runtime
	ctx     *quickjs.Context
}

func newQuickjsGoSession() (engineSession, error) {
	runtime := quickjs.NewRuntime()
	ctx := runtime.NewContext()
	return &quickjsGoSession{runtime: runtime, ctx: ctx}, nil
}

func (s *quickjsGoSession) evalInt(script string) (int64, error) {
	value := s.ctx.Eval(script)
	if value == nil {
		return 0, fmt.Errorf("quickjs-go eval returned nil")
	}
	defer value.Free()
	if value.IsException() || s.ctx.HasException() {
		return 0, s.ctx.Exception()
	}
	return value.Int64(), nil
}

func (s *quickjsGoSession) evalString(script string) (string, error) {
	value := s.ctx.Eval(script)
	if value == nil {
		return "", fmt.Errorf("quickjs-go eval returned nil")
	}
	defer value.Free()
	if value.IsException() || s.ctx.HasException() {
		return "", s.ctx.Exception()
	}
	return value.String(), nil
}

func (s *quickjsGoSession) close() {
	if s.ctx != nil {
		s.ctx.Close()
	}
	if s.runtime != nil {
		s.runtime.Close()
	}
}

func supportedEngines() []engine {
	return []engine{
		{
			id:      "goja",
			display: "GOJA（pure Go）",
			open:    newGojaSession,
		},
		{
			id:      "moderncquickjs",
			display: "ModerncQuickJS（ccgo / QuickJS）",
			open:    newModerncQuickJSSession,
		},
		{
			id:      "qjs",
			display: "QJS（Wasm / wazero）",
			open:    newQJSSession,
		},
		{
			id:      "v8go",
			display: "V8go（cgo / V8 JIT）",
			open:    newV8goSession,
		},
		{
			id:      "quickjs-go",
			display: "quickjs-go（cgo / QuickJS-ng）",
			open:    newQuickjsGoSession,
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
	return script
}

func wrapQJSCompatScript(script string) string {
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
