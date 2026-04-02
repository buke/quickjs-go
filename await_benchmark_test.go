package quickjs

import (
	"fmt"
	"testing"
	"time"
)

func benchmarkAwaitPollSliceSetTimeout(b *testing.B, pollSliceMs int, delayMs int) {
	original := GetAwaitPollSliceMs()
	SetAwaitPollSliceMs(pollSliceMs)
	b.Cleanup(func() {
		SetAwaitPollSliceMs(original)
	})

	rt := NewRuntime(WithModuleImport(true), WithExecuteTimeout(5))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	code := fmt.Sprintf(`new Promise((resolve) => { setTimeout(() => resolve(true), %d); })`, delayMs)
	var totalLatency time.Duration

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		result := ctx.Eval(code, EvalAwait(true))
		latency := time.Since(start)
		totalLatency += latency

		if result.IsException() {
			err := ctx.Exception()
			result.Free()
			b.Fatalf("await failed: %v", err)
		}
		result.Free()
	}
	b.StopTimer()

	if b.N > 0 {
		b.ReportMetric(float64(totalLatency.Microseconds())/float64(b.N), "us/await")
	}
}

func BenchmarkAwaitPollSliceSetTimeout(b *testing.B) {
	slices := []int{10, 5, 2}
	for _, pollSliceMs := range slices {
		name := fmt.Sprintf("slice_%dms", pollSliceMs)
		b.Run(name, func(b *testing.B) {
			benchmarkAwaitPollSliceSetTimeout(b, pollSliceMs, 20)
		})
	}
}
