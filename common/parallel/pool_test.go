package parallel

import (
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
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

func TestInfinitePool(t *testing.T) {
	pool := &InfinitePool{}
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
