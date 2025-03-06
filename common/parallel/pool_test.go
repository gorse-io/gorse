package parallel

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSequentialPool(t *testing.T) {
	pool := NewSequentialPool()
	count := 0
	for i := 0; i < 100; i++ {
		pool.Run(func() {
			count++
		})
	}
	pool.Wait()
	assert.Equal(t, 100, count)
}

func TestConcurrentPool(t *testing.T) {
	pool := NewConcurrentPool(1000)
	raceCount := 0
	atomicCount := atomic.Int64{}
	for i := 0; i < 1000; i++ {
		pool.Run(func() {
			raceCount++
			atomicCount.Add(1)
		})
	}
	pool.Wait()
	assert.Less(t, raceCount, 1000)
	assert.Equal(t, int64(1000), atomicCount.Load())
}
