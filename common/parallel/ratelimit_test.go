package parallel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnlimited(t *testing.T) {
	rateLimiter := &Unlimited{}
	assert.Zero(t, rateLimiter.Take(1))
}

func TestInitEmbeddingLimiters(t *testing.T) {
	InitEmbeddingLimiters(120, 180)
	assert.Equal(t, time.Duration(0), EmbeddingRequestsLimiter.Take(1))
	assert.InDelta(t, time.Second, EmbeddingRequestsLimiter.Take(2), float64(time.Millisecond))
	assert.Equal(t, time.Duration(0), EmbeddingTokensLimiter.Take(2))
	assert.InDelta(t, 2*time.Second, EmbeddingTokensLimiter.Take(5), float64(time.Millisecond))
}

func TestInitChatCompletionLimiters(t *testing.T) {
	InitChatCompletionLimiters(120, 180)
	assert.Equal(t, time.Duration(0), ChatCompletionRequestsLimiter.Take(1))
	assert.InDelta(t, time.Second, ChatCompletionRequestsLimiter.Take(2), float64(time.Millisecond))
	assert.Equal(t, time.Duration(0), ChatCompletionTokensLimiter.Take(2))
	assert.InDelta(t, 2*time.Second, ChatCompletionTokensLimiter.Take(5), float64(time.Millisecond))
}
