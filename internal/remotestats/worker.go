package remotestats

import (
	"context"
	"sync"
	"time"
)

const (
	// The first reading waits for the shell to start, which a heavy login
	// script can slow; later ones are a second apart.
	firstReadingTimeout = 10 * time.Second
	readingTimeout      = 5 * time.Second
)

// Result is one completed sample. An error is terminal for the worker so a
// refused or unsupported command is never retried in the background.
type Result struct {
	Sample Sample
	Err    error
}

// Worker owns one goroutine and one remote command only while the user has
// monitoring enabled.
type Worker struct {
	runner  Runner
	notify  func()
	ctx     context.Context
	cancel  context.CancelFunc
	results chan Result
	done    chan struct{}
	close   sync.Once
}

// Start begins sampling immediately. It is intentionally the constructor:
// merely opening a connection creates no worker, timer, or remote command.
func Start(runner Runner, notify func()) *Worker {
	return start(runner, notify, firstReadingTimeout, readingTimeout)
}

func start(runner Runner, notify func(), first, next time.Duration) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	worker := &Worker{
		runner: runner, notify: notify, ctx: ctx, cancel: cancel,
		results: make(chan Result, 1), done: make(chan struct{}),
	}
	go worker.run(first, next)
	return worker
}

// Results is the completion stream drained by the window thread. It holds only
// the latest unread result: a window that is slow to draw skips readings rather
// than making the server's command wait.
func (worker *Worker) Results() <-chan Result { return worker.results }

// Done closes once all sampling work has stopped.
func (worker *Worker) Done() <-chan struct{} { return worker.done }

// Close interrupts the running command and returns immediately.
func (worker *Worker) Close() { worker.close.Do(worker.cancel) }

func (worker *Worker) run(first, next time.Duration) {
	defer close(worker.done)
	defer close(worker.results)

	ctx, cancel := context.WithCancel(worker.ctx)
	defer cancel()
	stalled := make(chan struct{})
	var stall sync.Once
	watchdog := time.AfterFunc(first, func() { stall.Do(func() { close(stalled) }); cancel() })
	defer watchdog.Stop()

	var (
		previous counters
		have     bool
		fault    error
		pending  []GPU // the GPU lines that came before the counters line they belong to
	)
	err := worker.runner.StreamCommand(ctx, command, maxLineBytes, func(line []byte) bool {
		if isGPULine(line) {
			pending = parseGPULine(pending, line)
			return true
		}
		current, err := parse(line)
		if err != nil {
			fault = err
			return false
		}
		watchdog.Reset(next)
		if have {
			if reading, ok := sample(previous, current); ok {
				reading.GPUs = pending
				worker.publish(Result{Sample: reading})
			}
		}
		previous, have = current, true
		pending = nil
		return true
	})
	if worker.ctx.Err() != nil {
		return
	}
	switch {
	case fault != nil:
		err = fault
	case isClosed(stalled):
		err = ErrStalled
	case err == nil:
		err = ErrEnded
	}
	worker.publish(Result{Err: err})
}

func isClosed(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// publish keeps the newest result in the one-slot channel. The worker is its
// only sender, so after dropping an unread result the send cannot block.
func (worker *Worker) publish(result Result) {
	for {
		select {
		case worker.results <- result:
			if worker.notify != nil {
				worker.notify()
			}
			return
		default:
		}
		select {
		case <-worker.results:
		default:
		}
	}
}
