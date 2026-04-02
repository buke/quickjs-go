package quickjs

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"
)

func processCPUTime() (time.Duration, error) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, err
	}

	user := time.Duration(ru.Utime.Sec)*time.Second + time.Duration(ru.Utime.Usec)*time.Microsecond
	system := time.Duration(ru.Stime.Sec)*time.Second + time.Duration(ru.Stime.Usec)*time.Microsecond
	return user + system, nil
}

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

func benchmarkAwaitIdleCPUUsage(b *testing.B, pollSliceMs int, delayMs int) {
	originalSlice := GetAwaitPollSliceMs()
	SetAwaitPollSliceMs(pollSliceMs)
	b.Cleanup(func() {
		SetAwaitPollSliceMs(originalSlice)
	})

	rt := NewRuntime(WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	code := fmt.Sprintf(`new Promise((resolve) => { setTimeout(() => resolve(true), %d); })`, delayMs)

	var totalWall time.Duration
	var totalCPU time.Duration

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cpuStart, err := processCPUTime()
		if err != nil {
			b.Fatalf("Getrusage start failed: %v", err)
		}

		start := time.Now()
		result := ctx.Eval(code, EvalAwait(true))
		wall := time.Since(start)

		cpuEnd, err := processCPUTime()
		if err != nil {
			result.Free()
			b.Fatalf("Getrusage end failed: %v", err)
		}

		if result.IsException() {
			err := ctx.Exception()
			result.Free()
			b.Fatalf("await failed: %v", err)
		}
		result.Free()

		totalWall += wall
		totalCPU += cpuEnd - cpuStart
	}
	b.StopTimer()

	if b.N > 0 {
		avgWallUs := float64(totalWall.Microseconds()) / float64(b.N)
		avgCPUUs := float64(totalCPU.Microseconds()) / float64(b.N)
		cpuPct := 0.0
		if totalWall > 0 {
			cpuPct = 100.0 * float64(totalCPU) / float64(totalWall)
		}
		b.ReportMetric(avgWallUs, "us/wall")
		b.ReportMetric(avgCPUUs, "us/cpu")
		b.ReportMetric(cpuPct, "cpu_pct")
	}
}

func BenchmarkAwaitIdleLoopCPUUsage(b *testing.B) {
	benchmarkAwaitIdleCPUUsage(b, 2, 40)
}

func benchmarkAwaitInterruptPendingLatency(b *testing.B, pollSliceMs int, targetDelay time.Duration, heartbeatMs int) {
	originalSlice := GetAwaitPollSliceMs()
	SetAwaitPollSliceMs(pollSliceMs)
	b.Cleanup(func() {
		SetAwaitPollSliceMs(originalSlice)
	})

	rt := NewRuntime(WithModuleImport(true))
	defer rt.Close()
	ctx := rt.NewContext()
	defer ctx.Close()

	code := fmt.Sprintf(`new Promise(() => {
		const tick = () => setTimeout(tick, %d);
		tick();
	})`, heartbeatMs)

	var totalElapsed time.Duration
	var totalOvershoot time.Duration
	var totalInterruptCalls int64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		interruptCalls := int64(0)
		rt.SetInterruptHandler(func() int {
			interruptCalls++
			if time.Since(start) >= targetDelay {
				return 1
			}
			return 0
		})

		result := ctx.Eval(code, EvalAwait(true))
		elapsed := time.Since(start)

		if !result.IsException() {
			result.Free()
			b.Fatalf("expected interrupt exception, got success")
		}
		err := ctx.Exception()
		result.Free()
		if !strings.Contains(err.Error(), "interrupted") {
			b.Fatalf("expected interrupted error, got: %v", err)
		}
		if interruptCalls == 0 {
			b.Fatalf("interrupt handler never called")
		}

		totalElapsed += elapsed
		if elapsed > targetDelay {
			totalOvershoot += elapsed - targetDelay
		}
		totalInterruptCalls += interruptCalls
	}
	b.StopTimer()
	rt.ClearInterruptHandler()

	if b.N > 0 {
		b.ReportMetric(float64(totalElapsed.Microseconds())/float64(b.N), "us/latency")
		b.ReportMetric(float64(totalOvershoot.Microseconds())/float64(b.N), "us/overshoot")
		b.ReportMetric(float64(totalInterruptCalls)/float64(b.N), "calls/op")
	}
}

func BenchmarkAwaitInterruptPendingLatency(b *testing.B) {
	benchmarkAwaitInterruptPendingLatency(b, 5, 15*time.Millisecond, 25)
}
