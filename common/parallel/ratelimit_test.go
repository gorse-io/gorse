package parallel

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNoRateLimiter(t *testing.T) {
	rateLimiter := &NoRateLimiter{}
	assert.Zero(t, rateLimiter.Take(1))
}

func TestInitLimiters(t *testing.T) {
	InitLimiters(2, 3)
	assert.Equal(t, time.Duration(0), TPMLimiter().Take(1))
	assert.InDelta(t, time.Minute, TPMLimiter().Take(2), float64(time.Millisecond))
	assert.Equal(t, time.Duration(0), RPMLimiter().Take(2))
	assert.InDelta(t, 2*time.Minute, RPMLimiter().Take(5), float64(time.Millisecond))
}
