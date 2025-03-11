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
	pool := NewConcurrentPool(100)
	count := atomic.Int64{}
	for i := 0; i < 100; i++ {
		pool.Run(func() {
			count.Add(1)
		})
	}
	pool.Wait()
	assert.Equal(t, int64(100), count.Load())
}
