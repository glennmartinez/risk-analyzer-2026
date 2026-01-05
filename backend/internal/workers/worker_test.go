package workers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultWorkerConfig(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")

	assert.Equal(t, "test-worker", config.WorkerName)
	assert.Equal(t, 3, config.Concurrency)
	assert.Equal(t, 2*time.Second, config.PollInterval)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.True(t, config.EnableRecovery)
}

func TestNewBaseWorker(t *testing.T) {
	config := DefaultWorkerConfig("base-worker")
	worker := NewBaseWorker(config)

	assert.NotNil(t, worker)
	assert.Equal(t, "base-worker", worker.Name())
	assert.False(t, worker.IsRunning())
}

func TestBaseWorker_Name(t *testing.T) {
	config := WorkerConfig{WorkerName: "my-worker"}
	worker := NewBaseWorker(config)

	assert.Equal(t, "my-worker", worker.Name())
}

func TestBaseWorker_IsRunning(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")
	worker := NewBaseWorker(config)

	assert.False(t, worker.IsRunning())

	worker.setRunning(true)
	assert.True(t, worker.IsRunning())

	worker.setRunning(false)
	assert.False(t, worker.IsRunning())
}

func TestBaseWorker_Stats(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")
	worker := NewBaseWorker(config)

	// Initial stats
	stats := worker.Stats()
	assert.Equal(t, "test-worker", stats.WorkerName)
	assert.Equal(t, int64(0), stats.JobsProcessed)
	assert.Equal(t, int64(0), stats.JobsSucceeded)
	assert.Equal(t, int64(0), stats.JobsFailed)
	assert.False(t, stats.IsRunning)

	// Start worker and record some jobs
	worker.setRunning(true)

	startTime := worker.recordJobStart()
	time.Sleep(10 * time.Millisecond)
	worker.recordJobSuccess(startTime)

	startTime = worker.recordJobStart()
	time.Sleep(10 * time.Millisecond)
	worker.recordJobFailure(startTime)

	// Check updated stats
	stats = worker.Stats()
	assert.Equal(t, int64(2), stats.JobsProcessed)
	assert.Equal(t, int64(1), stats.JobsSucceeded)
	assert.Equal(t, int64(1), stats.JobsFailed)
	assert.Greater(t, stats.AverageProcessTime, time.Duration(0))
	assert.True(t, stats.IsRunning)
	assert.Greater(t, stats.Uptime, time.Duration(0))
}

func TestBaseWorker_RecordJobSuccess(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")
	worker := NewBaseWorker(config)

	startTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	worker.recordJobSuccess(startTime)

	stats := worker.Stats()
	assert.Equal(t, int64(1), stats.JobsProcessed)
	assert.Equal(t, int64(1), stats.JobsSucceeded)
	assert.Equal(t, int64(0), stats.JobsFailed)
	assert.Greater(t, stats.AverageProcessTime, time.Duration(0))
	assert.False(t, stats.LastJobTime.IsZero())
}

func TestBaseWorker_RecordJobFailure(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")
	worker := NewBaseWorker(config)

	startTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	worker.recordJobFailure(startTime)

	stats := worker.Stats()
	assert.Equal(t, int64(1), stats.JobsProcessed)
	assert.Equal(t, int64(0), stats.JobsSucceeded)
	assert.Equal(t, int64(1), stats.JobsFailed)
	assert.Greater(t, stats.AverageProcessTime, time.Duration(0))
	assert.False(t, stats.LastJobTime.IsZero())
}

func TestBaseWorker_ResetStats(t *testing.T) {
	config := DefaultWorkerConfig("test-worker")
	worker := NewBaseWorker(config)

	// Record some jobs
	startTime := worker.recordJobStart()
	worker.recordJobSuccess(startTime)

	// Verify stats are not zero
	stats := worker.Stats()
	assert.Greater(t, stats.JobsProcessed, int64(0))

	// Reset stats
	worker.resetStats()

	// Verify stats are reset
	stats = worker.Stats()
	assert.Equal(t, int64(0), stats.JobsProcessed)
	assert.Equal(t, int64(0), stats.JobsSucceeded)
	assert.Equal(t, int64(0), stats.JobsFailed)
	assert.Equal(t, time.Duration(0), stats.AverageProcessTime)
	assert.True(t, stats.LastJobTime.IsZero())
}

func TestBaseWorker_Config(t *testing.T) {
	config := WorkerConfig{
		WorkerName:  "test-worker",
		Concurrency: 5,
	}
	worker := NewBaseWorker(config)

	returnedConfig := worker.Config()
	assert.Equal(t, config.WorkerName, returnedConfig.WorkerName)
	assert.Equal(t, config.Concurrency, returnedConfig.Concurrency)
}

func TestBaseWorker_ConcurrentAccess(t *testing.T) {
	config := DefaultWorkerConfig("concurrent-worker")
	worker := NewBaseWorker(config)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startTime := worker.recordJobStart()
			worker.recordJobSuccess(startTime)
		}()
	}

	wg.Wait()

	// Verify stats
	stats := worker.Stats()
	assert.Equal(t, int64(iterations), stats.JobsProcessed)
	assert.Equal(t, int64(iterations), stats.JobsSucceeded)
}

func TestNewWorkerPool(t *testing.T) {
	pool := NewWorkerPool()
	assert.NotNil(t, pool)
	assert.Equal(t, 0, pool.Count())
}

func TestWorkerPool_AddWorker(t *testing.T) {
	pool := NewWorkerPool()

	worker := NewMockWorker("worker-1")

	pool.AddWorker(worker)
	assert.Equal(t, 1, pool.Count())

	// Add another worker
	worker2 := NewMockWorker("worker-2")
	pool.AddWorker(worker2)
	assert.Equal(t, 2, pool.Count())
}

func TestWorkerPool_GetWorker(t *testing.T) {
	pool := NewWorkerPool()

	worker := NewMockWorker("worker-1")
	pool.AddWorker(worker)

	// Get existing worker
	found := pool.GetWorker("worker-1")
	assert.NotNil(t, found)
	assert.Equal(t, "worker-1", found.Name())

	// Get non-existent worker
	notFound := pool.GetWorker("non-existent")
	assert.Nil(t, notFound)
}

func TestWorkerPool_ListWorkers(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	pool.AddWorker(worker1)

	worker2 := NewMockWorker("worker-2")
	pool.AddWorker(worker2)

	workers := pool.ListWorkers()
	assert.Len(t, workers, 2)

	names := make(map[string]bool)
	for _, w := range workers {
		names[w.Name()] = true
	}
	assert.True(t, names["worker-1"])
	assert.True(t, names["worker-2"])
}

func TestWorkerPool_GetAllStats(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	pool.AddWorker(worker1)

	worker2 := NewMockWorker("worker-2")
	pool.AddWorker(worker2)

	stats := pool.GetAllStats()
	assert.Len(t, stats, 2)

	// Verify stats structure
	for _, s := range stats {
		assert.NotEmpty(t, s.WorkerName)
	}
}

func TestWorkerPool_Count(t *testing.T) {
	pool := NewWorkerPool()
	assert.Equal(t, 0, pool.Count())

	worker := NewMockWorker("worker-1")
	pool.AddWorker(worker)
	assert.Equal(t, 1, pool.Count())
}

func TestRecoverableJobProcessor(t *testing.T) {
	t.Run("normal execution", func(t *testing.T) {
		called := false
		processor := func(ctx context.Context, job interface{}) error {
			called = true
			return nil
		}

		recoverable := RecoverableJobProcessor(processor)
		err := recoverable(context.Background(), "test-job")

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("panic recovery", func(t *testing.T) {
		processor := func(ctx context.Context, job interface{}) error {
			panic("test panic")
		}

		recoverable := RecoverableJobProcessor(processor)
		err := recoverable(context.Background(), "test-job")

		assert.Error(t, err)
		assert.IsType(t, &WorkerPanicError{}, err)
	})

	t.Run("error propagation", func(t *testing.T) {
		processor := func(ctx context.Context, job interface{}) error {
			return assert.AnError
		}

		recoverable := RecoverableJobProcessor(processor)
		err := recoverable(context.Background(), "test-job")

		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestWorkerError(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		err := NewWorkerError("worker-1", "start", nil, "custom message")
		assert.Equal(t, "custom message", err.Error())
		assert.Nil(t, err.Unwrap())
	})

	t.Run("with wrapped error", func(t *testing.T) {
		wrappedErr := assert.AnError
		err := NewWorkerError("worker-1", "process", wrappedErr, "")
		assert.Contains(t, err.Error(), "worker-1:process")
		assert.Contains(t, err.Error(), wrappedErr.Error())
		assert.Equal(t, wrappedErr, err.Unwrap())
	})

	t.Run("minimal error", func(t *testing.T) {
		err := NewWorkerError("worker-1", "stop", nil, "")
		assert.Equal(t, "worker-1:stop: unknown error", err.Error())
	})
}

func TestWorkerPanicError(t *testing.T) {
	t.Run("string panic", func(t *testing.T) {
		err := &WorkerPanicError{Panic: "string panic"}
		assert.Contains(t, err.Error(), "worker panic")
		assert.Contains(t, err.Error(), "string panic")
	})

	t.Run("error panic", func(t *testing.T) {
		err := &WorkerPanicError{Panic: assert.AnError}
		assert.Contains(t, err.Error(), "worker panic")
	})

	t.Run("unknown panic", func(t *testing.T) {
		err := &WorkerPanicError{Panic: 123}
		assert.Contains(t, err.Error(), "worker panic")
		assert.Contains(t, err.Error(), "unknown panic")
	})
}

// Mock Worker for testing pool operations
type MockWorker struct {
	name      string
	running   bool
	startErr  error
	stopErr   error
	mu        sync.RWMutex
	startedAt time.Time
}

func NewMockWorker(name string) *MockWorker {
	return &MockWorker{name: name}
}

func (w *MockWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.startErr != nil {
		return w.startErr
	}
	w.running = true
	w.startedAt = time.Now()
	return nil
}

func (w *MockWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopErr != nil {
		return w.stopErr
	}
	w.running = false
	return nil
}

func (w *MockWorker) Name() string {
	return w.name
}

func (w *MockWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *MockWorker) Stats() WorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var uptime time.Duration
	if !w.startedAt.IsZero() {
		uptime = time.Since(w.startedAt)
	}

	return WorkerStats{
		WorkerName: w.name,
		IsRunning:  w.running,
		Uptime:     uptime,
	}
}

func TestWorkerPool_StartAll(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	worker2 := NewMockWorker("worker-2")

	pool.AddWorker(worker1)
	pool.AddWorker(worker2)

	ctx := context.Background()
	err := pool.StartAll(ctx)
	require.NoError(t, err)

	assert.True(t, worker1.IsRunning())
	assert.True(t, worker2.IsRunning())
}

func TestWorkerPool_StartAll_Error(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	worker1.startErr = assert.AnError

	pool.AddWorker(worker1)

	ctx := context.Background()
	err := pool.StartAll(ctx)
	assert.Error(t, err)
}

func TestWorkerPool_StopAll(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	worker2 := NewMockWorker("worker-2")

	pool.AddWorker(worker1)
	pool.AddWorker(worker2)

	ctx := context.Background()

	// Start workers
	err := pool.StartAll(ctx)
	require.NoError(t, err)

	// Stop workers
	err = pool.StopAll(ctx)
	require.NoError(t, err)

	assert.False(t, worker1.IsRunning())
	assert.False(t, worker2.IsRunning())
}

func TestWorkerPool_StopAll_Error(t *testing.T) {
	pool := NewWorkerPool()

	worker1 := NewMockWorker("worker-1")
	worker1.stopErr = assert.AnError

	pool.AddWorker(worker1)

	ctx := context.Background()
	err := pool.StopAll(ctx)
	assert.Error(t, err)
}

func TestWorkerPool_ConcurrentAccess(t *testing.T) {
	pool := NewWorkerPool()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent adds
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker := NewMockWorker(fmt.Sprintf("worker-%d", id))
			pool.AddWorker(worker)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, iterations, pool.Count())

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.ListWorkers()
			_ = pool.GetAllStats()
		}()
	}

	wg.Wait()
}

func TestWorkerStats_Serialization(t *testing.T) {
	stats := WorkerStats{
		WorkerName:         "test-worker",
		JobsProcessed:      100,
		JobsSucceeded:      95,
		JobsFailed:         5,
		AverageProcessTime: 100 * time.Millisecond,
		LastJobTime:        time.Now(),
		Uptime:             1 * time.Hour,
		IsRunning:          true,
	}

	assert.Equal(t, "test-worker", stats.WorkerName)
	assert.Equal(t, int64(100), stats.JobsProcessed)
	assert.Equal(t, int64(95), stats.JobsSucceeded)
	assert.Equal(t, int64(5), stats.JobsFailed)
	assert.True(t, stats.IsRunning)
}
