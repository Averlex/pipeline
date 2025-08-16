// Package pipeline implements a simple pipeline engine based on Pipeline Pattern.
package pipeline

// Stage declares a function signature which is expected for each pipeline's stage.
type Stage func(in <-chan any) (out <-chan any)

// ExecutePipeline starts a concurrent pipeline following the Pipeline Pattern.
//
// The function also receives `done` channel for a possibility to stop the pipeline from elsewhere.
//
// It also drains all input channels (each stage's one and the source pipeline channel) on receiving
// a stop signal to prevent goroutine blocking (internal ones as well as external ones).
//
// nil channel, received instead of `done` is a valid scenario - this way, no external control is expected,
// so the pipeline works until the input channel is closed.
//
// Returns nil if the input channel is nil or no stages defined.
// Returns the output channel on success.
func ExecutePipeline(in <-chan any, done <-chan struct{}, stages ...Stage) <-chan any {
	if in == nil || len(stages) == 0 {
		return nil
	}

	prevStageChan := in

	for _, stage := range stages {
		prevStageChan = runStage(stage, prevStageChan, done)
	}

	return prevStageChan
}

// runStage runs the given stage. The stage is working until the input channel is closed
// or the stop signal received (if any at all).
//
// The functions drains the input channel if it receives the stop signal.
func runStage(stage Stage, in <-chan any, done <-chan struct{}) <-chan any {
	transit := make(chan any)

	go func() {
		if done == nil {
			ch := make(chan struct{})
			defer close(ch)
			done = ch
		}

		defer close(transit)

		// Draining the channel in case of stop signal.
		defer func() {
			go func() {
				for range in {
					continue
				}
			}()
		}()

		for {
			select {
			case <-done:
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case <-done:
					return
				case transit <- v:
				}
			}
		}
	}()

	return stage(transit)
}
