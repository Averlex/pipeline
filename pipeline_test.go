package pipeline

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	sleepPerStage = time.Millisecond * 100
	fault         = sleepPerStage / 2
)

func TestPipeline_CommonTests(t *testing.T) {
	// Local stage generator.
	g := func(_ string, f func(v any) any) Stage {
		return func(in <-chan any) <-chan any {
			out := make(chan any)
			go func() {
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}

	stages := []Stage{
		g("Dummy", func(v any) any { return v }),
		g("Multiplier (* 2)", func(v any) any { return v.(int) * 2 }),
		g("Adder (+ 100)", func(v any) any { return v.(int) + 100 }),
		g("Stringifier", func(v any) any { return strconv.Itoa(v.(int)) }),
	}

	t.Run("simple case", func(t *testing.T) {
		in := make(chan any)
		data := []int{1, 2, 3, 4, 5}

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, nil, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		require.Equal(t, []string{"102", "104", "106", "108", "110"}, result)
		require.Less(t,
			int64(elapsed),
			// ~0.8s for processing 5 values in 4 stages (100ms every) concurrently.
			int64(sleepPerStage)*int64(len(stages)+len(data)-1)+int64(fault))
	})

	t.Run("done case", func(t *testing.T) {
		in := make(chan any)
		done := make(chan struct{})
		data := []int{1, 2, 3, 4, 5}

		// Abort after 200ms.
		abortDur := sleepPerStage * 2
		go func() {
			<-time.After(abortDur)
			close(done)
		}()

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, done, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		require.Len(t, result, 0)
		require.Less(t, int64(elapsed), int64(abortDur)+int64(fault))
	})
}

func TestPipeline_AllStageStop(t *testing.T) {
	wg := sync.WaitGroup{}
	// Local stage generator.
	g := func(_ string, f func(v any) any) Stage {
		return func(in <-chan any) <-chan any {
			out := make(chan any)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}

	stages := []Stage{
		g("Dummy", func(v any) any { return v }),
		g("Multiplier (* 2)", func(v any) any { return v.(int) * 2 }),
		g("Adder (+ 100)", func(v any) any { return v.(int) + 100 }),
		g("Stringifier", func(v any) any { return strconv.Itoa(v.(int)) }),
	}

	t.Run("done case", func(t *testing.T) {
		in := make(chan any)
		done := make(chan struct{})
		data := []int{1, 2, 3, 4, 5}

		// Abort after 200ms.
		abortDur := sleepPerStage * 2
		go func() {
			<-time.After(abortDur)
			close(done)
		}()

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		for s := range ExecutePipeline(in, done, stages...) {
			result = append(result, s.(string))
		}
		wg.Wait()

		require.Len(t, result, 0)
	})
}

func TestPipeline_IncorrectArguments(t *testing.T) {
	wg := &sync.WaitGroup{}

	in := make(chan any)
	defer close(in)

	testCases := []struct {
		name   string
		in     chan any
		done   chan struct{}
		stages []Stage
	}{
		{"nil input channel", nil, nil, []Stage{g(wg, func(v any) any { return v })}},
		{"empty stages slice", in, nil, nil},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			got := ExecutePipeline(tC.in, tC.done, tC.stages...)
			require.Nil(t, got, "unexpected not-nil channel received from the pipeline")
		})
	}

	wg.Wait()
}

// Stage generator.
func g(wg *sync.WaitGroup, f func(v any) any) Stage {
	return func(in <-chan any) <-chan any {
		out := make(chan any)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(out)
			for v := range in {
				time.Sleep(sleepPerStage)
				out <- f(v)
			}
		}()
		return out
	}
}

func TestPipeline_LongPipeline(t *testing.T) {
	wg := &sync.WaitGroup{}

	in := make(chan any)
	data := make([]int, 100)
	for i := range data {
		data[i] = i
	}

	stages := make([]Stage, 100)
	for i := range stages {
		stages[i] = g(wg, func(v any) any { return v.(int) + 1 })
	}

	go func() {
		defer close(in)
		for _, v := range data {
			in <- v
		}
	}()

	res := make([]int, 0, len(data))
	for s := range ExecutePipeline(in, nil, stages...) {
		res = append(res, s.(int))
	}

	require.Equal(t, len(data), len(res))
	require.Equal(t, sum(data)+len(stages)*len(data), sum(res))

	wg.Wait()
}

func sum[T any](arr []T) int {
	var res int
	for _, v := range arr {
		res += any(v).(int)
	}
	return res
}
