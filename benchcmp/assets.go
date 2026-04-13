package main

import (
	"embed"
	"fmt"
	"strings"
)

const factorialExpected = 3628800

var v8V7FileOrder = []string{
	"base.js",
	"richards.js",
	"deltablue.js",
	"crypto.js",
	"raytrace.js",
	"earley-boyer.js",
	"regexp.js",
	"splay.js",
	"navier-stokes.js",
}

var v8V7MetricOrder = []string{
	"Richards",
	"DeltaBlue",
	"Crypto",
	"RayTrace",
	"EarleyBoyer",
	"RegExp",
	"Splay",
	"NavierStokes",
	"Score (version 7)",
	"Duration (seconds)",
}

//go:embed assets/README.md assets/LICENSE assets/v8-v7/*
var assetFS embed.FS

func factorialSource(loopCount int) string {
	return fmt.Sprintf(`
function factorial(n) {
  return n <= 1 ? 1 : n * factorial(n - 1);
}

let result = 0;
for (let i = 0; i < %d; i++) {
  result = factorial(10);
}

if (result !== %d) {
  throw new Error("unexpected factorial result: " + result);
}

result;
`, loopCount, factorialExpected)
}

func v8V7Source() (string, error) {
	var builder strings.Builder
	builder.WriteString(v8V7PreludeScript())

	scripts, err := v8V7AssetScripts()
	if err != nil {
		return "", err
	}

	for _, script := range scripts {
		builder.WriteString(script)
		builder.WriteString("\n")
	}

	builder.WriteString(v8V7RunnerScript())

	return builder.String(), nil
}

func v8V7AssetScripts() ([]string, error) {
	scripts := make([]string, 0, len(v8V7FileOrder))
	for _, name := range v8V7FileOrder {
		content, err := assetFS.ReadFile("assets/v8-v7/" + name)
		if err != nil {
			return nil, fmt.Errorf("read v8-v7 asset %s: %w", name, err)
		}
		scripts = append(scripts, string(content))
	}
	return scripts, nil
}

func v8V7PreludeScript() string {
	return "var __bench_output = [];\nfunction print(value) { __bench_output.push(String(value)); }\n"
}

func v8V7RunnerScript() string {
	return `
var __bench_success = true;

function __bench_print_result(name, result) {
  print(name + ': ' + result);
}

function __bench_print_error(name, error) {
  __bench_print_result(name, error);
  __bench_success = false;
}

function __bench_print_score(score) {
  if (__bench_success) {
    print('----');
    print('Score (version ' + BenchmarkSuite.version + '): ' + score);
  }
}

BenchmarkSuite.RunSuites({
  NotifyResult: __bench_print_result,
  NotifyError: __bench_print_error,
  NotifyScore: __bench_print_score
});

JSON.stringify(__bench_output);
`
}
