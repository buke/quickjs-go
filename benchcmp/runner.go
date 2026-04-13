package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Engines             []string
	Suites              []string
	FactorialIterations int
	FactorialLoops      int
}

type factorialResult struct {
	engine    engine
	durations []time.Duration
}

type v8V7Result struct {
	engine   engine
	metrics  map[string]string
	duration time.Duration
}

func Run(w io.Writer, config Config) error {
	if config.FactorialIterations <= 0 {
		return fmt.Errorf("factorial iterations must be positive")
	}
	if config.FactorialLoops <= 0 {
		return fmt.Errorf("factorial loops must be positive")
	}

	engines, err := selectEngines(config.Engines)
	if err != nil {
		return err
	}

	suites := normalizeSuites(config.Suites)
	if len(suites) == 0 {
		return fmt.Errorf("no suites selected")
	}

	first := true
	for _, suite := range suites {
		if !first {
			fmt.Fprintln(w)
		}
		first = false

		switch suite {
		case "factorial":
			results, err := runFactorial(engines, config.FactorialIterations, config.FactorialLoops)
			if err != nil {
				return err
			}
			writeFactorialTable(w, results, config.FactorialLoops)
		case "v8-v7":
			results, err := runV8V7(engines)
			if err != nil {
				return err
			}
			writeV8V7Table(w, results)
		default:
			return fmt.Errorf("unsupported suite %q", suite)
		}
	}

	return nil
}

func normalizeSuites(suites []string) []string {
	result := make([]string, 0, len(suites))
	seen := map[string]struct{}{}
	for _, suite := range suites {
		normalized := strings.ToLower(strings.TrimSpace(suite))
		switch normalized {
		case "v8v7":
			normalized = "v8-v7"
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func runFactorial(engines []engine, iterations int, loopCount int) ([]factorialResult, error) {
	source := factorialSource(loopCount)
	results := make([]factorialResult, 0, len(engines))

	for _, item := range engines {
		result := factorialResult{
			engine:    item,
			durations: make([]time.Duration, 0, iterations),
		}
		for run := 0; run < iterations; run++ {
			start := time.Now()
			value, err := item.evalInt(source)
			duration := time.Since(start)
			if err != nil {
				return nil, fmt.Errorf("%s factorial run %d: %w", item.display, run+1, err)
			}
			if value != factorialExpected {
				return nil, fmt.Errorf("%s factorial run %d: expected %d, got %d", item.display, run+1, factorialExpected, value)
			}
			result.durations = append(result.durations, duration)
		}
		results = append(results, result)
	}

	return results, nil
}

func runV8V7(engines []engine) ([]v8V7Result, error) {
	source, err := v8V7Source()
	if err != nil {
		return nil, err
	}

	results := make([]v8V7Result, 0, len(engines))
	for _, item := range engines {
		start := time.Now()
		var (
			output string
			err    error
		)
		if item.runV8V7 != nil {
			output, err = item.runV8V7()
		} else {
			output, err = item.evalString(source)
		}
		duration := time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("%s v8-v7: %w", item.display, err)
		}

		metrics, err := parseV8V7Output(output)
		if err != nil {
			return nil, fmt.Errorf("%s v8-v7 parse: %w", item.display, err)
		}
		metrics["Duration (seconds)"] = fmt.Sprintf("%.3fs", duration.Seconds())

		results = append(results, v8V7Result{
			engine:   item,
			metrics:  metrics,
			duration: duration,
		})
	}

	return results, nil
}

func parseV8V7Output(output string) (map[string]string, error) {
	var lines []string
	if err := json.Unmarshal([]byte(output), &lines); err != nil {
		return nil, fmt.Errorf("decode output: %w", err)
	}

	metrics := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "----" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected output line %q", line)
		}
		metrics[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	for _, name := range v8V7MetricOrder[:len(v8V7MetricOrder)-1] {
		if _, ok := metrics[name]; !ok {
			return nil, fmt.Errorf("missing metric %q", name)
		}
	}

	return metrics, nil
}

func writeFactorialTable(w io.Writer, results []factorialResult, loopCount int) {
	if len(results) == 0 {
		return
	}

	fmt.Fprintln(w, "## Factorial Calculation")
	fmt.Fprintf(w, "\nComputing factorial(10) %s times\n\n", formatInt(loopCount))

	headers := []string{"Iteration"}
	for _, result := range results {
		headers = append(headers, result.engine.display)
	}
	writeMarkdownRow(w, headers)
	writeMarkdownSeparator(w, len(headers))

	iterations := len(results[0].durations)
	for index := 0; index < iterations; index++ {
		row := []string{fmt.Sprintf("%d", index+1)}
		for _, result := range results {
			row = append(row, formatDuration(result.durations[index]))
		}
		writeMarkdownRow(w, row)
	}

	type summary struct {
		average time.Duration
		total   time.Duration
	}

	summaries := make([]summary, len(results))
	for index, result := range results {
		summaries[index] = summary{
			average: averageDuration(result.durations),
			total:   sumDurations(result.durations),
		}
	}

	fastestAverage := summaries[0].average
	fastestTotal := summaries[0].total
	for _, item := range summaries[1:] {
		if item.average < fastestAverage {
			fastestAverage = item.average
		}
		if item.total < fastestTotal {
			fastestTotal = item.total
		}
	}

	baselineAverage, baselineName, hasBaseline := firstFactorialBaseline(results)

	averageRow := []string{"Average"}
	totalRow := []string{"Total"}
	relativeRowLabel := "Speed"
	if hasBaseline {
		relativeRowLabel = "Speed vs " + baselineName
	}
	relativeRow := []string{relativeRowLabel}
	baselineIndex := 0
	for index, item := range summaries {
		averageRow = append(averageRow, emphasizeIfFastest(formatDuration(item.average), item.average == fastestAverage))
		totalRow = append(totalRow, emphasizeIfFastest(formatDuration(item.total), item.total == fastestTotal))
		if hasBaseline {
			relativeRow = append(relativeRow, emphasizeIfBaseline(fmt.Sprintf("%.2fx", float64(baselineAverage)/float64(item.average)), index == baselineIndex))
		} else {
			relativeRow = append(relativeRow, emphasizeIfFastest(fmt.Sprintf("%.2fx", float64(item.average)/float64(fastestAverage)), item.average == fastestAverage))
		}
	}
	writeMarkdownRow(w, averageRow)
	writeMarkdownRow(w, totalRow)
	writeMarkdownRow(w, relativeRow)
}

func writeV8V7Table(w io.Writer, results []v8V7Result) {
	fmt.Fprintln(w, "## AreWeFastYet V8-V7")
	fmt.Fprintln(w)

	headers := []string{"Metric"}
	for _, result := range results {
		headers = append(headers, result.engine.display)
	}
	writeMarkdownRow(w, headers)
	writeMarkdownSeparator(w, len(headers))

	for _, metric := range v8V7MetricOrder {
		row := []string{metric}
		bestIndexes := bestMetricIndexes(results, metric)
		for index, result := range results {
			value := result.metrics[metric]
			if metric == "Duration (seconds)" {
				row = append(row, emphasizeIfBest(value, containsIndex(bestIndexes, index)))
				continue
			}
			row = append(row, emphasizeIfBest(value, containsIndex(bestIndexes, index)))
		}
		writeMarkdownRow(w, row)
	}

	baselineScore, baselineDuration, baselineName, hasBaseline := firstV8V7Baseline(results)
	if hasBaseline {
		scoreRow := []string{"Score vs " + baselineName}
		runtimeRow := []string{"Speed vs " + baselineName}
		for index, result := range results {
			score := parseMetricValue(result.metrics["Score (version 7)"])
			scoreRow = append(scoreRow, emphasizeIfBaseline(fmt.Sprintf("%.2fx", score/baselineScore), index == 0))
			runtimeRow = append(runtimeRow, emphasizeIfBaseline(fmt.Sprintf("%.2fx", float64(baselineDuration)/float64(result.duration)), index == 0))
		}
		writeMarkdownRow(w, scoreRow)
		writeMarkdownRow(w, runtimeRow)
	}
}

func firstFactorialBaseline(results []factorialResult) (time.Duration, string, bool) {
	if len(results) == 0 {
		return 0, "", false
	}
	return averageDuration(results[0].durations), results[0].engine.display, true
}

func firstV8V7Baseline(results []v8V7Result) (float64, time.Duration, string, bool) {
	if len(results) == 0 {
		return 0, 0, "", false
	}
	return parseMetricValue(results[0].metrics["Score (version 7)"]), results[0].duration, results[0].engine.display, true
}

func bestMetricIndexes(results []v8V7Result, metric string) []int {
	type candidate struct {
		index int
		value float64
	}

	candidates := make([]candidate, 0, len(results))
	for index, result := range results {
		value := result.metrics[metric]
		parsed := parseMetricValue(value)
		candidates = append(candidates, candidate{index: index, value: parsed})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if metric == "Duration (seconds)" {
			return candidates[i].value < candidates[j].value
		}
		return candidates[i].value > candidates[j].value
	})

	if len(candidates) == 0 {
		return nil
	}

	best := []int{candidates[0].index}
	for _, candidate := range candidates[1:] {
		if candidate.value == candidates[0].value {
			best = append(best, candidate.index)
		}
	}
	return best
}

func parseMetricValue(value string) float64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "s"))
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func containsIndex(indexes []int, target int) bool {
	for _, index := range indexes {
		if index == target {
			return true
		}
	}
	return false
}

func writeMarkdownRow(w io.Writer, columns []string) {
	fmt.Fprint(w, "|")
	for _, column := range columns {
		fmt.Fprintf(w, " %s |", column)
	}
	fmt.Fprintln(w)
}

func writeMarkdownSeparator(w io.Writer, count int) {
	columns := make([]string, count)
	for index := range columns {
		columns[index] = "---"
	}
	writeMarkdownRow(w, columns)
}

func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	return sumDurations(durations) / time.Duration(len(durations))
}

func sumDurations(durations []time.Duration) time.Duration {
	var total time.Duration
	for _, duration := range durations {
		total += duration
	}
	return total
}

func formatDuration(duration time.Duration) string {
	if duration >= time.Second {
		return fmt.Sprintf("%.3fs", duration.Seconds())
	}
	return fmt.Sprintf("%.3fms", float64(duration)/float64(time.Millisecond))
}

func emphasizeIfFastest(value string, fastest bool) string {
	if fastest {
		return "**" + value + "**"
	}
	return value
}

func emphasizeIfBest(value string, best bool) string {
	if best {
		return "**" + value + "**"
	}
	return value
}

func emphasizeIfBaseline(value string, baseline bool) string {
	if baseline {
		return "**" + value + "**"
	}
	return value
}

func formatInt(value int) string {
	decimal := fmt.Sprintf("%d", value)
	if len(decimal) <= 3 {
		return decimal
	}

	var parts []string
	for len(decimal) > 3 {
		parts = append(parts, decimal[len(decimal)-3:])
		decimal = decimal[:len(decimal)-3]
	}
	parts = append(parts, decimal)

	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, ",")
}
