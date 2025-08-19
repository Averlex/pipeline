# pipeline

[![Go version](https://img.shields.io/badge/go-1.24.2+-blue.svg)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/Averlex/pipeline.svg)](https://pkg.go.dev/github.com/Averlex/pipeline)
[![Go Report Card](https://goreportcard.com/badge/github.com/Averlex/pipeline)](https://goreportcard.com/report/github.com/Averlex/pipeline)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A composable pipeline execution engine for Go with graceful shutdown via done channel.

## Features

- ✅ Chain multiple processing stages
- ✅ Graceful shutdown via `done` channel
- ✅ Safe handling of `nil` `done` — no external control
- ✅ Input channels are drained on stop to prevent goroutine leaks
- ✅ Concurrent execution
- ✅ Simple, idiomatic Go interface

## Usage

```go
import "github.com/Averlex/pipeline"

// Define stages.
double := func(in <-chan any) <-chan any {
    out := make(chan any)
    go func() {
        defer close(out)
        for v := range in {
            if n, ok := v.(int); ok {
                out <- n * 2
            }
        }
    }()
    return out
}

toString := func(in <-chan any) <-chan any {
    out := make(chan any)
    go func() {
        defer close(out)
        for v := range in {
            out <- fmt.Sprintf("value: %v", v)
        }
    }()
    return out
}

// Prepare input.
in := make(chan any)
go func() {
    defer close(in)
    for i := 1; i <= 3; i++ {
        in <- i
    }
}()

// Optional: create done signal to stop early.
done := make(chan struct{})
// close(done) // to terminate pipeline.

// Execute pipeline
out := pipeline.ExecutePipeline(in, done, double, toString)

// Read results.
for v := range out {
    fmt.Println(v) // "value: 2", "value: 4", "value: 6"
}
```

## Interface

```go
type Stage func(in <-chan any) <-chan any

func ExecutePipeline(
    in <-chan any,
    done <-chan struct{},
    stages ...Stage,
) <-chan any
```

- `in`: input channel. If `nil`, returns `nil`.
- `done`: signal to stop the pipeline early.
  If `nil`, the pipeline runs until input is closed (no external control).
- `stages`: variadic list of processing functions.
- **Returns**: output channel, or `nil` if in is `nil` or no stages provided.

## Behavior

- Each stage runs concurrently.
- If `done` is closed, the pipeline stops and drains all input channels to prevent blocking.
- The returned output channel is closed when the last stage finishes.

## Type Safety

> ⚠️ **Note**: This pipeline uses `any`, so type safety between stages is not enforced at compile time.
> Ensure that the output type of one stage matches the input type of the next.

For example, if stage 1 returns `string`, stage 2 must accept `string`.

## Installation

```bash
go get github.com/Averlex/pipeline
```
