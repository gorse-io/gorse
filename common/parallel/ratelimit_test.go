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
	InitEmbeddingLimiters(2, 3)
	assert.Equal(t, time.Duration(0), EmbeddingRequestsLimiter.Take(1))
	assert.InDelta(t, time.Minute, EmbeddingRequestsLimiter.Take(2), float64(time.Millisecond))
	assert.Equal(t, time.Duration(0), EmbeddingTokensLimiter.Take(2))
	assert.InDelta(t, 2*time.Minute, EmbeddingTokensLimiter.Take(5), float64(time.Millisecond))
}

func TestInitChatCompletionLimiters(t *testing.T) {
	InitChatCompletionLimiters(2, 3)
	assert.Equal(t, time.Duration(0), ChatCompletionRequestsLimiter.Take(1))
	assert.InDelta(t, time.Minute, ChatCompletionRequestsLimiter.Take(2), float64(time.Millisecond))
	assert.Equal(t, time.Duration(0), ChatCompletionTokensLimiter.Take(2))
	assert.InDelta(t, 2*time.Minute, ChatCompletionTokensLimiter.Take(5), float64(time.Millisecond))
}

func TestBackOff_Factor(t *testing.T) {
	backOff := NewBackOff()
	assert.Equal(t, 1, backOff.Factor())
	backOff.BackOff()
	assert.Equal(t, 2, backOff.Factor())
	backOff.BackOff()
	assert.Equal(t, 4, backOff.Factor())
	backOff.Recover()
	assert.Equal(t, 3, backOff.Factor())
}
