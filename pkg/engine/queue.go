package engine

import (
	"pipeliner/pkg/logger"
	"sync"

	"github.com/sirupsen/logrus"
)

// EngineQueue manages concurrent scan execution with a simple semaphore
type EngineQueue struct {
	semaphore chan struct{}
	running   int
	queued    int
	mu        sync.Mutex
	logger    *logger.Logger
}

var (
	globalQueue *EngineQueue
	queueOnce   sync.Once
)

// InitGlobalQueue initializes the global engine queue with max concurrency
func InitGlobalQueue(maxConcurrent int) {
	queueOnce.Do(func() {
		if maxConcurrent < 1 {
			maxConcurrent = 1
		}
		globalQueue = &EngineQueue{
			semaphore: make(chan struct{}, maxConcurrent),
			running:   0,
			queued:    0,
			logger:    logger.NewLogger(logrus.InfoLevel),
		}
		globalQueue.logger.Info("Scan queue initialized", logger.Fields{
			"max_concurrent": maxConcurrent,
		})
	})
}

// GetGlobalQueue returns the global queue instance (initializes with default if needed)
func GetGlobalQueue() *EngineQueue {
	if globalQueue == nil {
		InitGlobalQueue(1)
	}
	return globalQueue
}

// ExecuteWithQueue wraps a function execution with queue management
// It blocks until a slot is available, then executes the function
func (q *EngineQueue) ExecuteWithQueue(fn func() error) error {
	q.mu.Lock()
	q.queued++
	currentQueued := q.queued
	currentRunning := q.running
	maxSlots := cap(q.semaphore)
	q.mu.Unlock()

	q.logger.Info("Scan added to queue", logger.Fields{
		"queued":  currentQueued,
		"running": currentRunning,
		"slots":   maxSlots,
	})

	q.semaphore <- struct{}{}

	q.mu.Lock()
	q.queued--
	q.running++
	finalQueued := q.queued
	finalRunning := q.running
	q.mu.Unlock()

	q.logger.Info("Scan execution started", logger.Fields{
		"running": finalRunning,
		"queued":  finalQueued,
	})

	defer func() {
		<-q.semaphore
		q.mu.Lock()
		q.running--
		remainingRunning := q.running
		remainingQueued := q.queued
		q.mu.Unlock()

		q.logger.Info("Scan execution completed, slot released", logger.Fields{
			"running": remainingRunning,
			"queued":  remainingQueued,
		})
	}()

	return fn()
}

// GetStatus returns current queue status
func (q *EngineQueue) GetStatus() (running, queued, maxConcurrent int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running, q.queued, cap(q.semaphore)
}
