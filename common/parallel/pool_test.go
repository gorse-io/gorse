package parallel

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSequentialPool(t *testing.T) {
	pool := &SequentialPool{}
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
	pool := &ConcurrentPool{}
	raceCount := 0
	atomicCount := atomic.Int64{}
	for i := 0; i < 100; i++ {
		pool.Run(func() {
			raceCount++
			atomicCount.Add(1)
		})
	}
	pool.Wait()
	assert.Less(t, raceCount, 100)
	assert.Equal(t, int64(100), atomicCount.Load())
}
