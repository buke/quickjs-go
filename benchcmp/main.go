package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	engines := flag.String("engines", "quickjs-go,goja,moderncquickjs,qjs,v8go", "comma-separated engine list")
	suites := flag.String("suites", "factorial,v8-v7", "comma-separated suite list")
	factorialIterations := flag.Int("factorial-iterations", 5, "number of measured factorial runs")
	factorialLoops := flag.Int("factorial-loops", 1000000, "number of factorial(10) executions per run")
	flag.Parse()

	config := Config{
		Engines:             splitCSV(*engines),
		Suites:              splitCSV(*suites),
		FactorialIterations: *factorialIterations,
		FactorialLoops:      *factorialLoops,
	}

	if err := Run(os.Stdout, config); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
