package quickjs

/*
#include "bridge.h"
#include <time.h>
*/
import "C"
import (
	"errors"
	"fmt"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// InterruptHandler is a function type for interrupt handler.
// Return != 0 if the JS code needs to be interrupted
type InterruptHandler func() int

type interruptHandlerHolder struct {
	fn InterruptHandler
}

// runtimeNewContextHook is used in tests to force JS_NewContext failure paths.
// It must remain nil in production.
var runtimeNewContextHook func(rt *C.JSRuntime) *C.JSContext

// runtimeInitContextHook is used in tests to force context initialization failure paths.
// It must remain nil in production.
var runtimeInitContextHook func(ctx *C.JSContext) C.JSValue

// runtimeEvalFunctionHook is used in tests to force JS_EvalFunction failure paths.
// It must remain nil in production.
var runtimeEvalFunctionHook func(ctx *C.JSContext, compiled C.JSValue) C.JSValue

// runtimeBootstrapStdOSHook and runtimeBootstrapTimersHook are used in tests
// to force bootstrap failure paths. They must remain nil in production.
var runtimeBootstrapStdOSHook func(ctx *Context) bool
var runtimeBootstrapTimersHook func(ctx *Context) bool
var runtimeApplyIntrinsicsHook func(ctx *C.JSContext, set IntrinsicSet) (handled bool, ok bool)
var runtimeApplyIntrinsicStepHook func(name string) (handled bool, ok bool)

// runtimeBootstrapStdOSInitHook is used in tests to force std/os init
// outcomes while keeping BootstrapStdOS owner/liveness checks active.
// Return (handled=true, ok=<result>) to override default C initialization.
var runtimeBootstrapStdOSInitHook func(ctx *Context) (handled bool, ok bool)

var errOwnerAccessDenied = errors.New("quickjs: owner access denied; runtime/context/value APIs must be called from the owner goroutine; if strict OS thread mode is enabled, also bind that goroutine with runtime.LockOSThread()")

var ownerCheckCurrentGoroutineID = currentGoroutineID
var ownerCheckCurrentThreadID = currentThreadID
var goroutineStack = goruntime.Stack

type classObjectIdentity struct {
	contextID uint64
	handleID  int32
}

// Runtime represents a Javascript runtime with simplified interrupt handling
type Runtime struct {
	mu                     sync.RWMutex
	ref                    *C.JSRuntime
	runtimeInfo            *C.char
	options                *Options
	ownerGoroutineID       atomic.Uint64
	ownerThreadID          atomic.Uint64
	interruptHandlerState  atomic.Pointer[interruptHandlerHolder]
	contexts               sync.Map
	contextsByID           sync.Map
	contextIDCounter       atomic.Uint64
	constructorRegistry    sync.Map
	classObjectRegistry    sync.Map
	classObjectIDsByCtx    sync.Map
	classObjectIDCounter   atomic.Int32
	closeOnce              sync.Once
	closed                 atomic.Bool
	stdHandlersInitialized bool
}

// isAlive reports whether the runtime still has a valid native handle and
// has not started closing.
func (r *Runtime) isAlive() bool {
	return r != nil && r.ref != nil && !r.closed.Load()
}

type Options struct {
	timeout              uint64
	memoryLimit          uint64
	gcThreshold          int64
	maxStackSize         uint64
	canBlock             bool
	moduleImport         bool
	strip                int
	ownerGoroutineCheck  bool
	strictThreadAffinity bool
}

type Option func(*Options)

// ContextBootstrapOptions controls host bootstrap for new contexts.
type ContextBootstrapOptions struct {
	loadStdOS    bool
	injectTimers bool
}

type ContextBootstrapOption func(*ContextBootstrapOptions)

// MemoryUsage mirrors QuickJS JSMemoryUsage fields.
type MemoryUsage struct {
	MallocSize         int64
	MallocLimit        int64
	MemoryUsedSize     int64
	MallocCount        int64
	MemoryUsedCount    int64
	AtomCount          int64
	AtomSize           int64
	StrCount           int64
	StrSize            int64
	ObjCount           int64
	ObjSize            int64
	PropCount          int64
	PropSize           int64
	ShapeCount         int64
	ShapeSize          int64
	JSFuncCount        int64
	JSFuncSize         int64
	JSFuncCodeSize     int64
	JSFuncPC2LineCount int64
	JSFuncPC2LineSize  int64
	CFuncCount         int64
	ArrayCount         int64
	FastArrayCount     int64
	FastArrayElements  int64
	BinaryObjectCount  int64
	BinaryObjectSize   int64
}

// IntrinsicSet controls which QuickJS intrinsics are injected into a raw context.
type IntrinsicSet struct {
	BaseObjects  bool
	Date         bool
	Eval         bool
	RegExp       bool
	JSON         bool
	Proxy        bool
	MapSet       bool
	TypedArrays  bool
	Promise      bool
	BigInt       bool
	WeakRef      bool
	Performance  bool
	DOMException bool
}

// IntrinsicOption modifies IntrinsicSet.
type IntrinsicOption func(*IntrinsicSet)

// NewIntrinsicSet builds an IntrinsicSet from options.
func NewIntrinsicSet(opts ...IntrinsicOption) IntrinsicSet {
	set := IntrinsicSet{}
	for _, opt := range opts {
		if opt != nil {
			opt(&set)
		}
	}
	return normalizeIntrinsicSet(set)
}

// AllIntrinsics enables all QuickJS intrinsics.
func AllIntrinsics() IntrinsicSet {
	return IntrinsicSet{
		BaseObjects:  true,
		Date:         true,
		Eval:         true,
		RegExp:       true,
		JSON:         true,
		Proxy:        true,
		MapSet:       true,
		TypedArrays:  true,
		Promise:      true,
		BigInt:       true,
		WeakRef:      true,
		Performance:  true,
		DOMException: true,
	}
}

// MinimalIntrinsics enables only base language objects.
func MinimalIntrinsics() IntrinsicSet {
	return IntrinsicSet{BaseObjects: true}
}

// WithBaseObjects toggles base object intrinsic injection.
func WithBaseObjects(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.BaseObjects = enabled }
}

// WithDate toggles Date intrinsic injection.
func WithDate(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.Date = enabled }
}

// WithEval toggles eval intrinsic injection.
func WithEval(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.Eval = enabled }
}

// WithRegExp toggles RegExp intrinsic injection.
func WithRegExp(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.RegExp = enabled }
}

// WithJSON toggles JSON intrinsic injection.
func WithJSON(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.JSON = enabled }
}

// WithProxy toggles Proxy intrinsic injection.
func WithProxy(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.Proxy = enabled }
}

// WithMapSet toggles Map/Set intrinsic injection.
func WithMapSet(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.MapSet = enabled }
}

// WithTypedArrays toggles typed-array intrinsic injection.
func WithTypedArrays(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.TypedArrays = enabled }
}

// WithPromise toggles Promise intrinsic injection.
func WithPromise(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.Promise = enabled }
}

// WithBigInt toggles BigInt intrinsic injection.
func WithBigInt(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.BigInt = enabled }
}

// WithWeakRef toggles WeakRef intrinsic injection.
func WithWeakRef(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.WeakRef = enabled }
}

// WithPerformance toggles performance intrinsic injection.
func WithPerformance(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.Performance = enabled }
}

// WithDOMException toggles DOMException intrinsic injection.
func WithDOMException(enabled bool) IntrinsicOption {
	return func(s *IntrinsicSet) { s.DOMException = enabled }
}

func normalizeIntrinsicSet(set IntrinsicSet) IntrinsicSet {
	if set.Date || set.Eval || set.RegExp || set.JSON || set.Proxy || set.MapSet ||
		set.TypedArrays || set.Promise || set.BigInt || set.WeakRef || set.Performance || set.DOMException {
		set.BaseObjects = true
	}
	return set
}

// DefaultBootstrap enables the same bootstrap pipeline as Runtime.NewContext:
// std/os module registration plus global timer injection.
func DefaultBootstrap() ContextBootstrapOption {
	return func(o *ContextBootstrapOptions) {
		o.loadStdOS = true
		o.injectTimers = true
	}
}

// MinimalBootstrap enables std/os module registration but skips timer injection.
func MinimalBootstrap() ContextBootstrapOption {
	return func(o *ContextBootstrapOptions) {
		o.loadStdOS = true
		o.injectTimers = false
	}
}

// NoBootstrap disables all host bootstrap steps.
func NoBootstrap() ContextBootstrapOption {
	return func(o *ContextBootstrapOptions) {
		o.loadStdOS = false
		o.injectTimers = false
	}
}

// WithBootstrapStdOS toggles std/os module registration in bootstrap.
func WithBootstrapStdOS(enabled bool) ContextBootstrapOption {
	return func(o *ContextBootstrapOptions) {
		o.loadStdOS = enabled
	}
}

// WithBootstrapTimers toggles timer injection in bootstrap.
//
// Timer injection imports setTimeout/clearTimeout from the "os" module, so
// enabling timers implicitly requires std/os registration. During option
// normalization, injectTimers=true forces loadStdOS=true.
func WithBootstrapTimers(enabled bool) ContextBootstrapOption {
	return func(o *ContextBootstrapOptions) {
		o.injectTimers = enabled
	}
}

func newContextBootstrapOptions(opts ...ContextBootstrapOption) ContextBootstrapOptions {
	cfg := ContextBootstrapOptions{
		loadStdOS:    true,
		injectTimers: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	// Timer bootstrap imports from "os", so keep std/os enabled when timers
	// are requested even if options were applied in a conflicting order.
	if cfg.injectTimers && !cfg.loadStdOS {
		cfg.loadStdOS = true
	}
	return cfg
}

const defaultTimerBootstrapCode = `
import { setTimeout, clearTimeout } from "os";
globalThis.setTimeout = setTimeout;
globalThis.clearTimeout = clearTimeout;
`

func (r *Runtime) ensureOwnerAccess() bool {
	if r == nil {
		return false
	}

	if r.options == nil || r.options.ownerGoroutineCheck {
		gid := ownerCheckCurrentGoroutineID()
		if gid == 0 {
			return false
		}
		if !r.claimOrVerifyOwnerGoroutine(gid) {
			return false
		}
	}

	if r.options != nil && r.options.strictThreadAffinity {
		tid := ownerCheckCurrentThreadID()
		if tid == 0 {
			return false
		}
		if !r.claimOrVerifyOwnerThread(tid) {
			return false
		}
	}

	return true
}

func (r *Runtime) claimOrVerifyOwnerGoroutine(current uint64) bool {
	owner := r.ownerGoroutineID.Load()
	if owner == 0 {
		return r.ownerGoroutineID.CompareAndSwap(0, current) || r.ownerGoroutineID.Load() == current
	}
	return owner == current
}

func (r *Runtime) claimOrVerifyOwnerThread(current uint64) bool {
	owner := r.ownerThreadID.Load()
	if owner == 0 {
		return r.ownerThreadID.CompareAndSwap(0, current) || r.ownerThreadID.Load() == current
	}
	return owner == current
}

func currentGoroutineID() uint64 {
	const prefix = "goroutine "

	var buf [128]byte
	n := goroutineStack(buf[:], false)
	if n <= len(prefix) {
		return 0
	}

	idx := len(prefix)
	var id uint64
	hasDigit := false
	for idx < n {
		c := buf[idx]
		if c < '0' || c > '9' {
			break
		}
		hasDigit = true
		id = id*10 + uint64(c-'0')
		idx++
	}

	if !hasDigit {
		return 0
	}
	return id
}

func currentThreadID() uint64 {
	return uint64(C.CurrentThreadID())
}

// WithExecuteTimeout will set the runtime's execute timeout; default is 0
func WithExecuteTimeout(timeout uint64) Option {
	return func(o *Options) {
		o.timeout = timeout
	}
}

// WithMemoryLimit will set the runtime memory limit; if not set, it will be unlimit.
func WithMemoryLimit(memoryLimit uint64) Option {
	return func(o *Options) {
		o.memoryLimit = memoryLimit
	}
}

// WithGCThreshold will set the runtime's GC threshold; default is -1 to disable automatic GC.
func WithGCThreshold(gcThreshold int64) Option {
	return func(o *Options) {
		o.gcThreshold = gcThreshold
	}
}

// WithMaxStackSize will set max runtime's stack size; default is 0 disable maximum stack size check
func WithMaxStackSize(maxStackSize uint64) Option {
	return func(o *Options) {
		o.maxStackSize = maxStackSize
	}
}

// WithCanBlock will set the runtime's can block; default is true
func WithCanBlock(canBlock bool) Option {
	return func(o *Options) {
		o.canBlock = canBlock
	}
}

func WithModuleImport(moduleImport bool) Option {
	return func(o *Options) {
		o.moduleImport = moduleImport
	}
}

func WithStripInfo(strip int) Option {
	return func(o *Options) {
		o.strip = strip
	}
}

// WithOwnerGoroutineCheck enables/disables owner-goroutine checks.
// WARNING: disabling this check is unsafe and may cause data races or memory corruption.
func WithOwnerGoroutineCheck(enabled bool) Option {
	return func(o *Options) {
		o.ownerGoroutineCheck = enabled
	}
}

// WithStrictOSThread enables strict OS-thread affinity checks.
func WithStrictOSThread(enabled bool) Option {
	return func(o *Options) {
		o.strictThreadAffinity = enabled
	}
}

// NewRuntime creates a new quickjs runtime with simplified interrupt handling.
func NewRuntime(opts ...Option) *Runtime {
	options := &Options{
		timeout:              0,
		memoryLimit:          0,
		gcThreshold:          -1,
		maxStackSize:         0,
		canBlock:             true,
		moduleImport:         false,
		strip:                1,
		ownerGoroutineCheck:  true,
		strictThreadAffinity: false,
	}
	for _, opt := range opts {
		opt(options)
	}

	rt := &Runtime{
		ref:     C.JS_NewRuntime(),
		options: options,
	}
	registerRuntime(rt.ref, rt)
	C.SetPromiseRejectionTracker(rt.ref, 1)

	// Configure runtime options
	if rt.options.memoryLimit > 0 {
		rt.SetMemoryLimit(rt.options.memoryLimit)
	}

	if rt.options.gcThreshold >= -1 {
		rt.SetGCThreshold(rt.options.gcThreshold)
	}

	rt.SetMaxStackSize(rt.options.maxStackSize)

	if rt.options.canBlock {
		C.JS_SetCanBlock(rt.ref, C.bool(true))
	}

	if rt.options.strip > 0 {
		rt.SetStripInfo(rt.options.strip)
	}

	if rt.options.moduleImport {
		rt.SetModuleImport(rt.options.moduleImport)
	}

	// Set timeout after other options (will override interrupt handler)
	if rt.options.timeout > 0 {
		rt.SetExecuteTimeout(rt.options.timeout)
	}

	return rt
}

// RunGC will call quickjs's garbage collector.
func (r *Runtime) RunGC() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_RunGC(r.ref)
}

// SetAwaitPollSliceMs configures AwaitValue idle poll slice duration in milliseconds.
// Values <= 0 are ignored.
func SetAwaitPollSliceMs(timeoutMs int) {
	if timeoutMs <= 0 {
		return
	}
	C.SetAwaitPollSliceMs(C.int(timeoutMs))
}

// GetAwaitPollSliceMs returns AwaitValue idle poll slice duration in milliseconds.
func GetAwaitPollSliceMs() int {
	return int(C.GetAwaitPollSliceMs())
}

// Close will free the runtime pointer with proper cleanup.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}

	r.closeOnce.Do(func() {
		r.closed.Store(true)

		var contexts []*Context
		r.contexts.Range(func(_, value interface{}) bool {
			if ctx, ok := value.(*Context); ok {
				contexts = append(contexts, ctx)
			}
			return true
		})
		for _, ctx := range contexts {
			ctx.Close()
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		if r.ref == nil {
			return
		}

		ref := r.ref
		r.interruptHandlerState.Store(nil)
		C.ClearInterruptHandler(ref)
		C.SetPromiseRejectionTracker(ref, 0)

		r.constructorRegistry.Range(func(key, _ interface{}) bool {
			r.constructorRegistry.Delete(key)
			return true
		})

		r.classObjectRegistry.Range(func(key, _ interface{}) bool {
			r.classObjectRegistry.Delete(key)
			return true
		})
		r.classObjectIDsByCtx.Range(func(key, _ interface{}) bool {
			r.classObjectIDsByCtx.Delete(key)
			return true
		})
		r.contextsByID.Range(func(key, _ interface{}) bool {
			r.contextsByID.Delete(key)
			return true
		})

		unregisterRuntime(ref)
		if r.stdHandlersInitialized {
			C.js_std_free_handlers(ref)
			r.stdHandlersInitialized = false
		}

		C.JS_FreeRuntime(ref)
		if r.runtimeInfo != nil {
			C.free(unsafe.Pointer(r.runtimeInfo))
			r.runtimeInfo = nil
		}
		r.ref = nil
	})
}

// SetCanBlock will set the runtime's can block; default is true
func (r *Runtime) SetCanBlock(canBlock bool) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}

	C.JS_SetCanBlock(r.ref, C.bool(canBlock))
}

// SetMemoryLimit the runtime memory limit; if not set, it will be unlimit.
func (r *Runtime) SetMemoryLimit(limit uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetMemoryLimit(r.ref, C.size_t(limit))
}

// SetGCThreshold the runtime's GC threshold; use -1 to disable automatic GC.
func (r *Runtime) SetGCThreshold(threshold int64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetGCThreshold(r.ref, C.size_t(threshold))
}

// GCThreshold returns the runtime GC threshold.
func (r *Runtime) GCThreshold() uint64 {
	if r == nil {
		return 0
	}
	if !r.ensureOwnerAccess() {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return 0
	}
	return uint64(C.JS_GetGCThreshold(r.ref))
}

// SetDumpFlags configures runtime dump flags.
func (r *Runtime) SetDumpFlags(flags uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetDumpFlags(r.ref, C.uint64_t(flags))
}

// DumpFlags returns runtime dump flags.
func (r *Runtime) DumpFlags() uint64 {
	if r == nil {
		return 0
	}
	if !r.ensureOwnerAccess() {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return 0
	}
	return uint64(C.JS_GetDumpFlags(r.ref))
}

// SetInfo sets runtime informational string.
func (r *Runtime) SetInfo(info string) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	if r.runtimeInfo != nil {
		C.free(unsafe.Pointer(r.runtimeInfo))
		r.runtimeInfo = nil
	}
	if info == "" {
		C.JS_SetRuntimeInfo(r.ref, nil)
		return
	}
	r.runtimeInfo = C.CString(info)
	C.JS_SetRuntimeInfo(r.ref, r.runtimeInfo)
}

// MemoryUsage returns runtime memory usage snapshot.
func (r *Runtime) MemoryUsage() MemoryUsage {
	if r == nil {
		return MemoryUsage{}
	}
	if !r.ensureOwnerAccess() {
		return MemoryUsage{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return MemoryUsage{}
	}

	var s C.JSMemoryUsage
	C.JS_ComputeMemoryUsage(r.ref, &s)

	return MemoryUsage{
		MallocSize:         int64(s.malloc_size),
		MallocLimit:        int64(s.malloc_limit),
		MemoryUsedSize:     int64(s.memory_used_size),
		MallocCount:        int64(s.malloc_count),
		MemoryUsedCount:    int64(s.memory_used_count),
		AtomCount:          int64(s.atom_count),
		AtomSize:           int64(s.atom_size),
		StrCount:           int64(s.str_count),
		StrSize:            int64(s.str_size),
		ObjCount:           int64(s.obj_count),
		ObjSize:            int64(s.obj_size),
		PropCount:          int64(s.prop_count),
		PropSize:           int64(s.prop_size),
		ShapeCount:         int64(s.shape_count),
		ShapeSize:          int64(s.shape_size),
		JSFuncCount:        int64(s.js_func_count),
		JSFuncSize:         int64(s.js_func_size),
		JSFuncCodeSize:     int64(s.js_func_code_size),
		JSFuncPC2LineCount: int64(s.js_func_pc2line_count),
		JSFuncPC2LineSize:  int64(s.js_func_pc2line_size),
		CFuncCount:         int64(s.c_func_count),
		ArrayCount:         int64(s.array_count),
		FastArrayCount:     int64(s.fast_array_count),
		FastArrayElements:  int64(s.fast_array_elements),
		BinaryObjectCount:  int64(s.binary_object_count),
		BinaryObjectSize:   int64(s.binary_object_size),
	}
}

// DumpMemoryUsage returns a human-readable memory usage summary.
func (r *Runtime) DumpMemoryUsage() string {
	if r == nil {
		return ""
	}
	if !r.ensureOwnerAccess() {
		return ""
	}
	r.mu.RLock()
	if r.closed.Load() || r.ref == nil {
		r.mu.RUnlock()
		return ""
	}
	r.mu.RUnlock()

	usage := r.MemoryUsage()
	var b strings.Builder
	fmt.Fprintf(&b, "malloc_size=%d\n", usage.MallocSize)
	fmt.Fprintf(&b, "malloc_limit=%d\n", usage.MallocLimit)
	fmt.Fprintf(&b, "memory_used_size=%d\n", usage.MemoryUsedSize)
	fmt.Fprintf(&b, "malloc_count=%d\n", usage.MallocCount)
	fmt.Fprintf(&b, "memory_used_count=%d\n", usage.MemoryUsedCount)
	fmt.Fprintf(&b, "atom_count=%d atom_size=%d\n", usage.AtomCount, usage.AtomSize)
	fmt.Fprintf(&b, "str_count=%d str_size=%d\n", usage.StrCount, usage.StrSize)
	fmt.Fprintf(&b, "obj_count=%d obj_size=%d\n", usage.ObjCount, usage.ObjSize)
	fmt.Fprintf(&b, "prop_count=%d prop_size=%d\n", usage.PropCount, usage.PropSize)
	fmt.Fprintf(&b, "shape_count=%d shape_size=%d\n", usage.ShapeCount, usage.ShapeSize)
	fmt.Fprintf(&b, "js_func_count=%d js_func_size=%d js_func_code_size=%d\n", usage.JSFuncCount, usage.JSFuncSize, usage.JSFuncCodeSize)
	fmt.Fprintf(&b, "js_func_pc2line_count=%d js_func_pc2line_size=%d\n", usage.JSFuncPC2LineCount, usage.JSFuncPC2LineSize)
	fmt.Fprintf(&b, "c_func_count=%d array_count=%d\n", usage.CFuncCount, usage.ArrayCount)
	fmt.Fprintf(&b, "fast_array_count=%d fast_array_elements=%d\n", usage.FastArrayCount, usage.FastArrayElements)
	fmt.Fprintf(&b, "binary_object_count=%d binary_object_size=%d", usage.BinaryObjectCount, usage.BinaryObjectSize)
	return b.String()
}

// SetMaxStackSize will set max runtime's stack size;
func (r *Runtime) SetMaxStackSize(stack_size uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetMaxStackSize(r.ref, C.size_t(stack_size))
}

// SetExecuteTimeout will set the runtime's execute timeout;
// This will override any user interrupt handler (expected behavior)
func (r *Runtime) SetExecuteTimeout(timeout uint64) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.SetExecuteTimeout(r.ref, C.time_t(timeout))
	// Clear user interrupt handler since timeout takes precedence
	r.interruptHandlerState.Store(nil)
}

// SetStripInfo sets the strip info for the runtime.
func (r *Runtime) SetStripInfo(strip int) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	// quickjs-ng does not expose a runtime-level JS_SetStripInfo API.
	_ = strip
}

// SetModuleImport sets whether the runtime supports module import.
func (r *Runtime) SetModuleImport(moduleImport bool) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	C.JS_SetModuleLoaderFunc2(r.ref, (*C.JSModuleNormalizeFunc)(unsafe.Pointer(nil)), (*C.JSModuleLoaderFunc2)(C.js_module_loader), (*C.JSModuleCheckSupportedImportAttributes)(C.js_module_check_attributes), unsafe.Pointer(nil))
}

// SetInterruptHandler sets a user interrupt handler using simplified approach.
// This will override any timeout handler (expected behavior)
func (r *Runtime) SetInterruptHandler(handler InterruptHandler) {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return
	}
	if handler != nil {
		r.interruptHandlerState.Store(&interruptHandlerHolder{fn: handler})
	} else {
		r.interruptHandlerState.Store(nil)
	}

	if handler != nil {
		// Simplified call - no handlerArgs complexity
		C.SetInterruptHandler(r.ref)
	} else {
		C.ClearInterruptHandler(r.ref)
	}
}

// ClearInterruptHandler clears the user interrupt handler
func (r *Runtime) ClearInterruptHandler() {
	if r == nil {
		return
	}
	if !r.ensureOwnerAccess() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ref == nil {
		return
	}
	r.interruptHandlerState.Store(nil)
	C.ClearInterruptHandler(r.ref)
}

// callInterruptHandler is called from C layer via runtime mapping (internal use)
func (r *Runtime) callInterruptHandler() int {
	if r == nil {
		return 0
	}
	holder := r.interruptHandlerState.Load()
	if holder != nil && holder.fn != nil {
		return holder.fn()
	}
	return 0 // No interrupt
}

// NewContext creates a new JavaScript context with default host bootstrap.
func (r *Runtime) NewContext() *Context {
	return r.NewContextWithOptions(DefaultBootstrap())
}

// NewBareContext creates a JavaScript context without host bootstrap.
func (r *Runtime) NewBareContext() *Context {
	return r.NewContextWithOptions(NoBootstrap())
}

// NewContextRaw creates a raw QuickJS context and applies selected intrinsics.
func (r *Runtime) NewContextRaw(intrinsics IntrinsicSet) *Context {
	if r == nil {
		return nil
	}
	if !r.ensureOwnerAccess() {
		return nil
	}

	set := normalizeIntrinsicSet(intrinsics)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return nil
	}

	if !r.stdHandlersInitialized {
		C.js_std_init_handlers(r.ref)
		r.stdHandlersInitialized = true
	}

	var ctxRef *C.JSContext
	if runtimeNewContextHook != nil {
		ctxRef = runtimeNewContextHook(r.ref)
	} else {
		ctxRef = C.JS_NewContextRaw(r.ref)
	}
	if ctxRef == nil {
		return nil
	}

	ctx := &Context{
		contextID:   r.nextContextID(),
		ref:         ctxRef,
		runtime:     r,
		handleStore: newHandleStore(),
	}
	ctx.initScheduler()

	registerContext(ctxRef, ctx)
	r.registerOwnedContext(ctx)

	applyOK := false
	if runtimeApplyIntrinsicsHook != nil {
		handled, ok := runtimeApplyIntrinsicsHook(ctxRef, set)
		if handled {
			applyOK = ok
		} else {
			applyOK = applyIntrinsics(ctxRef, set)
		}
	} else {
		applyOK = applyIntrinsics(ctxRef, set)
	}

	if !applyOK {
		ctx.Close()
		return nil
	}

	return ctx
}

func applyIntrinsics(ctxRef *C.JSContext, set IntrinsicSet) bool {
	applyStep := func(name string, enabled bool, fn func() C.int) bool {
		if !enabled {
			return true
		}
		if runtimeApplyIntrinsicStepHook != nil {
			handled, ok := runtimeApplyIntrinsicStepHook(name)
			if handled {
				return ok
			}
		}
		return fn() >= 0
	}
	return applyStep("BaseObjects", set.BaseObjects, func() C.int { return C.JS_AddIntrinsicBaseObjects(ctxRef) }) &&
		applyStep("Date", set.Date, func() C.int { return C.JS_AddIntrinsicDate(ctxRef) }) &&
		applyStep("Eval", set.Eval, func() C.int { return C.JS_AddIntrinsicEval(ctxRef) }) &&
		applyStep("RegExp", set.RegExp, func() C.int {
			C.JS_AddIntrinsicRegExpCompiler(ctxRef)
			return C.JS_AddIntrinsicRegExp(ctxRef)
		}) &&
		applyStep("JSON", set.JSON, func() C.int { return C.JS_AddIntrinsicJSON(ctxRef) }) &&
		applyStep("Proxy", set.Proxy, func() C.int { return C.JS_AddIntrinsicProxy(ctxRef) }) &&
		applyStep("MapSet", set.MapSet, func() C.int { return C.JS_AddIntrinsicMapSet(ctxRef) }) &&
		applyStep("TypedArrays", set.TypedArrays, func() C.int { return C.JS_AddIntrinsicTypedArrays(ctxRef) }) &&
		applyStep("Promise", set.Promise, func() C.int { return C.JS_AddIntrinsicPromise(ctxRef) }) &&
		applyStep("BigInt", set.BigInt, func() C.int { return C.JS_AddIntrinsicBigInt(ctxRef) }) &&
		applyStep("WeakRef", set.WeakRef, func() C.int { return C.JS_AddIntrinsicWeakRef(ctxRef) }) &&
		applyStep("Performance", set.Performance, func() C.int { return C.JS_AddPerformance(ctxRef) }) &&
		applyStep("DOMException", set.DOMException, func() C.int { return C.JS_AddIntrinsicDOMException(ctxRef) })
}

// BootstrapStdOS registers std/os modules for the given context.
func BootstrapStdOS(ctx *Context) bool {
	if runtimeBootstrapStdOSHook != nil {
		return runtimeBootstrapStdOSHook(ctx)
	}
	if ctx == nil || ctx.ref == nil || ctx.runtime == nil {
		return false
	}
	if !ctx.runtime.ensureOwnerAccess() || !ctx.runtime.isAlive() {
		return false
	}

	stdModuleName := C.CString("std")
	defer C.free(unsafe.Pointer(stdModuleName))
	osModuleName := C.CString("os")
	defer C.free(unsafe.Pointer(osModuleName))
	if !initStdOSModules(ctx, stdModuleName, osModuleName) {
		return false
	}
	return true
}

func initStdOSModules(ctx *Context, stdModuleName *C.char, osModuleName *C.char) bool {
	if runtimeBootstrapStdOSInitHook != nil {
		handled, ok := runtimeBootstrapStdOSInitHook(ctx)
		if handled {
			return ok
		}
	}
	return C.js_init_module_std(ctx.ref, stdModuleName) != nil && C.js_init_module_os(ctx.ref, osModuleName) != nil
}

// BootstrapTimers injects setTimeout/clearTimeout into globalThis.
func BootstrapTimers(ctx *Context) bool {
	if runtimeBootstrapTimersHook != nil {
		return runtimeBootstrapTimersHook(ctx)
	}
	if ctx == nil || ctx.ref == nil || ctx.runtime == nil {
		return false
	}
	if !ctx.runtime.ensureOwnerAccess() || !ctx.runtime.isAlive() {
		return false
	}
	return initializeContextGlobals(ctx.ref, defaultTimerBootstrapCode, "init.js")
}

// NewContextWithOptions creates a JavaScript context with configurable host bootstrap.
func (r *Runtime) NewContextWithOptions(opts ...ContextBootstrapOption) *Context {
	if r == nil {
		return nil
	}
	if !r.ensureOwnerAccess() {
		return nil
	}
	bootstrap := newContextBootstrapOptions(opts...)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() || r.ref == nil {
		return nil
	}

	if !r.stdHandlersInitialized {
		C.js_std_init_handlers(r.ref)
		r.stdHandlersInitialized = true
	}

	// create a new context (heap, global object and context stack
	var ctx_ref *C.JSContext
	if runtimeNewContextHook != nil {
		ctx_ref = runtimeNewContextHook(r.ref)
	} else {
		ctx_ref = C.JS_NewContext(r.ref)
	}
	if ctx_ref == nil {
		return nil
	}

	// Create Context with HandleStore for function management
	ctx := &Context{
		contextID:   r.nextContextID(),
		ref:         ctx_ref,
		runtime:     r,
		handleStore: newHandleStore(), // Initialize HandleStore for function management
	}
	ctx.initScheduler()

	// Register context mapping for C callbacks
	registerContext(ctx_ref, ctx)
	r.registerOwnedContext(ctx)

	if bootstrap.loadStdOS && !BootstrapStdOS(ctx) {
		ctx.Close()
		return nil
	}
	if bootstrap.injectTimers && !BootstrapTimers(ctx) {
		ctx.Close()
		return nil
	}

	return ctx
}

func (r *Runtime) registerOwnedContext(ctx *Context) {
	if r == nil || ctx == nil || ctx.ref == nil {
		return
	}
	r.contexts.Store(ctx.ref, ctx)
	if ctx.contextID != 0 {
		r.contextsByID.Store(ctx.contextID, ctx)
	}
}

func (r *Runtime) unregisterOwnedContext(ctxRef *C.JSContext, contextID uint64) {
	if r == nil {
		return
	}
	if ctxRef != nil {
		r.contexts.Delete(ctxRef)
	}
	if contextID != 0 {
		r.contextsByID.Delete(contextID)
		r.cleanupClassObjectIdentitiesByContext(contextID)
	}
}

func (r *Runtime) nextContextID() uint64 {
	if r == nil {
		return 0
	}
	for {
		id := r.contextIDCounter.Add(1)
		if id != 0 {
			return id
		}
	}
}

func (r *Runtime) nextClassObjectID() int32 {
	if r == nil {
		return 0
	}
	id := r.classObjectIDCounter.Add(1)
	if id <= 0 {
		return 0
	}
	return -id
}

func (r *Runtime) getOwnedContextByID(contextID uint64) *Context {
	if r == nil || contextID == 0 {
		return nil
	}
	v, ok := r.contextsByID.Load(contextID)
	if !ok {
		return nil
	}
	ctx, ok := v.(*Context)
	if !ok {
		r.contextsByID.Delete(contextID)
		return nil
	}
	if !ctx.hasValidRef() {
		return nil
	}
	return ctx
}

func (r *Runtime) registerClassObjectIdentity(contextID uint64, handleID int32) int32 {
	if r == nil || contextID == 0 || handleID <= 0 {
		return 0
	}

	objectID := r.nextClassObjectID()
	if objectID == 0 {
		return 0
	}
	identity := classObjectIdentity{contextID: contextID, handleID: handleID}
	r.classObjectRegistry.Store(objectID, identity)

	bucketValue, _ := r.classObjectIDsByCtx.LoadOrStore(contextID, &sync.Map{})
	bucket, ok := bucketValue.(*sync.Map)
	if !ok {
		// Corruption path: serialize replacement to avoid concurrent overwrite.
		r.mu.Lock()
		currentBucket, loaded := r.classObjectIDsByCtx.Load(contextID)
		if loaded {
			if existing, typed := currentBucket.(*sync.Map); typed {
				bucket = existing
				ok = true
			}
		}
		if !ok {
			replacement := &sync.Map{}
			r.classObjectIDsByCtx.Store(contextID, replacement)
			bucket = replacement
		}
		r.mu.Unlock()
	}
	bucket.Store(objectID, struct{}{})

	return objectID
}

func (r *Runtime) getClassObjectIdentity(objectID int32) (classObjectIdentity, bool) {
	if r == nil || objectID == 0 {
		return classObjectIdentity{}, false
	}
	v, ok := r.classObjectRegistry.Load(objectID)
	if !ok {
		return classObjectIdentity{}, false
	}
	identity, ok := v.(classObjectIdentity)
	if !ok {
		r.classObjectRegistry.Delete(objectID)
		return classObjectIdentity{}, false
	}
	if identity.contextID == 0 || identity.handleID <= 0 {
		r.classObjectRegistry.Delete(objectID)
		return classObjectIdentity{}, false
	}
	return identity, true
}

func (r *Runtime) takeClassObjectIdentity(objectID int32) (classObjectIdentity, bool) {
	if r == nil || objectID == 0 {
		return classObjectIdentity{}, false
	}
	v, ok := r.classObjectRegistry.LoadAndDelete(objectID)
	if !ok {
		return classObjectIdentity{}, false
	}
	identity, ok := v.(classObjectIdentity)
	if !ok {
		return classObjectIdentity{}, false
	}
	if identity.contextID == 0 || identity.handleID <= 0 {
		return classObjectIdentity{}, false
	}

	if bucketValue, ok := r.classObjectIDsByCtx.Load(identity.contextID); ok {
		if bucket, ok := bucketValue.(*sync.Map); ok {
			bucket.Delete(objectID)
		}
	}

	return identity, true
}

func (r *Runtime) cleanupClassObjectIdentitiesByContext(contextID uint64) {
	if r == nil || contextID == 0 {
		return
	}
	bucketValue, ok := r.classObjectIDsByCtx.LoadAndDelete(contextID)
	if !ok {
		return
	}
	bucket, ok := bucketValue.(*sync.Map)
	if !ok {
		return
	}
	bucket.Range(func(key, _ interface{}) bool {
		objectID, ok := key.(int32)
		if !ok {
			return true
		}
		r.classObjectRegistry.Delete(objectID)
		return true
	})
}

func (r *Runtime) registerConstructorClassID(constructor C.JSValue, classID uint32) {
	if r == nil {
		return
	}
	r.constructorRegistry.Store(jsValueToKey(constructor), classID)
}

func (r *Runtime) getConstructorClassID(constructor C.JSValue) (uint32, bool) {
	if r == nil {
		return 0, false
	}

	constructorKey := jsValueToKey(constructor)
	if classIDInterface, ok := r.constructorRegistry.Load(constructorKey); ok {
		classID, ok := classIDInterface.(uint32)
		if !ok {
			r.constructorRegistry.Delete(constructorKey)
			return 0, false
		}
		return classID, true
	}
	return 0, false
}

func timeoutOpaqueCount() int {
	return int(C.GetTimeoutOpaqueCount())
}

func initializeContextGlobals(ctx *C.JSContext, code string, filename string) bool {
	if runtimeInitContextHook != nil {
		initRun := runtimeInitContextHook(ctx)
		if bool(C.JS_IsException(initRun)) {
			C.JS_FreeValue(ctx, initRun)
			return false
		}
		C.JS_FreeValue(ctx, initRun)
		return true
	}

	codeBuf := zeroTerminatedBytes(code)
	codePtr := (*C.char)(unsafe.Pointer(&codeBuf[0]))
	filenameBuf := zeroTerminatedBytes(filename)
	filenamePtr := (*C.char)(unsafe.Pointer(&filenameBuf[0]))

	var pinner goruntime.Pinner
	pinner.Pin(&codeBuf[0])
	pinner.Pin(&filenameBuf[0])
	defer pinner.Unpin()

	evalFlags := C.int(C.JS_EVAL_TYPE_MODULE) | C.int(C.JS_EVAL_FLAG_COMPILE_ONLY)
	initCompile := C.JS_Eval(ctx, codePtr, C.size_t(len(code)), filenamePtr, evalFlags)
	goruntime.KeepAlive(codeBuf)
	goruntime.KeepAlive(filenameBuf)
	if bool(C.JS_IsException(initCompile)) {
		C.JS_FreeValue(ctx, initCompile)
		return false
	}

	initEval := initCompile
	if runtimeEvalFunctionHook != nil {
		initEval = runtimeEvalFunctionHook(ctx, initCompile)
	} else {
		initEval = C.JS_EvalFunction(ctx, initCompile)
	}
	if bool(C.JS_IsException(initEval)) {
		C.JS_FreeValue(ctx, initEval)
		return false
	}

	initRun := C.AwaitValue(ctx, initEval)
	if bool(C.JS_IsException(initRun)) {
		C.JS_FreeValue(ctx, initRun)
		return false
	}

	C.JS_FreeValue(ctx, initRun)
	return true
}

func forceRuntimeNewContextFailureForTest(enable bool) func() {
	oldHook := runtimeNewContextHook
	if enable {
		runtimeNewContextHook = func(rt *C.JSRuntime) *C.JSContext {
			return nil
		}
	} else {
		runtimeNewContextHook = nil
	}
	return func() {
		runtimeNewContextHook = oldHook
	}
}

func forceRuntimeInitFailureForTest(enable bool) func() {
	oldHook := runtimeInitContextHook
	if enable {
		runtimeInitContextHook = func(ctx *C.JSContext) C.JSValue {
			msg := C.CString("forced init failure")
			defer C.free(unsafe.Pointer(msg))
			return C.ThrowInternalError(ctx, msg)
		}
	} else {
		runtimeInitContextHook = nil
	}
	return func() {
		runtimeInitContextHook = oldHook
	}
}

func forceRuntimeInitSuccessForTest(enable bool) func() {
	oldHook := runtimeInitContextHook
	if enable {
		runtimeInitContextHook = func(ctx *C.JSContext) C.JSValue {
			return C.JS_NewUndefined()
		}
	} else {
		runtimeInitContextHook = nil
	}
	return func() {
		runtimeInitContextHook = oldHook
	}
}

func forceRuntimeApplyIntrinsicsFailureForTest(enable bool) func() {
	oldHook := runtimeApplyIntrinsicsHook
	if enable {
		runtimeApplyIntrinsicsHook = func(ctx *C.JSContext, set IntrinsicSet) (bool, bool) {
			return true, false
		}
	} else {
		runtimeApplyIntrinsicsHook = nil
	}
	return func() {
		runtimeApplyIntrinsicsHook = oldHook
	}
}

func forceRuntimeApplyIntrinsicsPassthroughHookForTest(enable bool) func() {
	oldHook := runtimeApplyIntrinsicsHook
	if enable {
		runtimeApplyIntrinsicsHook = func(ctx *C.JSContext, set IntrinsicSet) (bool, bool) {
			return false, false
		}
	} else {
		runtimeApplyIntrinsicsHook = nil
	}
	return func() {
		runtimeApplyIntrinsicsHook = oldHook
	}
}

func forceRuntimeApplyIntrinsicStepFailureForTest(step string) func() {
	oldHook := runtimeApplyIntrinsicStepHook
	runtimeApplyIntrinsicStepHook = func(name string) (bool, bool) {
		if name == step {
			return true, false
		}
		return false, false
	}
	return func() {
		runtimeApplyIntrinsicStepHook = oldHook
	}
}

func forceRuntimeEvalFailureForTest(enable bool) func() {
	oldHook := runtimeEvalFunctionHook
	if enable {
		runtimeEvalFunctionHook = func(ctx *C.JSContext, compiled C.JSValue) C.JSValue {
			C.JS_FreeValue(ctx, compiled)
			msg := C.CString("forced eval failure")
			defer C.free(unsafe.Pointer(msg))
			return C.ThrowInternalError(ctx, msg)
		}
	} else {
		runtimeEvalFunctionHook = nil
	}
	return func() {
		runtimeEvalFunctionHook = oldHook
	}
}
