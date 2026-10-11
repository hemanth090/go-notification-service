package main

import (
	"log/slog"
	"sync"
	"time"
)

// workerGroup keeps track of the background delivery goroutines.
var workerGroup sync.WaitGroup

// startWorkers launches `totalWorkers` workers. Each one takes notifications
// off the queue and delivers them, one after another, until the queue is closed.
func startWorkers(totalWorkers int, queue <-chan notification) {
	for workerNumber := 0; workerNumber < totalWorkers; workerNumber++ {
		workerGroup.Add(1)
		go func(workerID int) {
			defer workerGroup.Done()
			slog.Info("worker started", "worker", workerID)
			for item := range queue {
				deliverWithRetry(item)
			}
			slog.Info("worker stopped", "worker", workerID)
		}(workerNumber)
	}
}

// waitForWorkers waits for all workers to finish, but gives up after `timeout`
// so shutdown can never hang forever.
func waitForWorkers(timeout time.Duration) {
	workersFinished := make(chan struct{})
	go func() {
		workerGroup.Wait()
		close(workersFinished)
	}()

	select {
	case <-workersFinished:
	case <-time.After(timeout):
		slog.Warn("workers did not finish in time")
	}
}

// deliverWithRetry tries to deliver a notification. If it fails, it waits a
// little and tries again, up to maxAttempts times.
func deliverWithRetry(item notification) {
	logger := slog.With("id", item.ID)

	var lastError error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Mark it as being tried right now.
		item.Attempts = attempt
		item.Status = statusPending
		database.save(item)

		lastError = deliver(item)
		if lastError == nil {
			item.Status = statusSent
			database.save(item)
			logger.Info("delivered", "attempts", attempt)
			return
		}

		logger.Warn("delivery failed", "attempt", attempt, "err", lastError)
		if attempt < maxAttempts {
			time.Sleep(backoff(attempt))
		}
	}

	// We ran out of tries. Mark it as failed and remember why.
	item.Status = statusFailed
	item.LastError = lastError.Error()
	database.save(item)
	logger.Error("gave up", "attempts", maxAttempts, "err", lastError)
}

// backoff returns how long to wait before retry number `attempt`.
// It doubles each time: base, then 2x, then 4x, ...
func backoff(attempt int) time.Duration {
	return 200 * time.Millisecond << (attempt - 1)
}
