package parallel

import (
	"time"

	"github.com/juju/ratelimit"
)

var (
	ChatCompletionRequestsLimiter RateLimiter = &Unlimited{}
	ChatCompletionTokensLimiter   RateLimiter = &Unlimited{}
	EmbeddingRequestsLimiter      RateLimiter = &Unlimited{}
	EmbeddingTokensLimiter        RateLimiter = &Unlimited{}
)

func InitChatCompletionLimiters(rpm, tpm int) {
	if rpm > 0 {
		ChatCompletionRequestsLimiter = ratelimit.NewBucketWithQuantum(time.Second, int64(rpm/60), int64(rpm/60))
	}
	if tpm > 0 {
		ChatCompletionTokensLimiter = ratelimit.NewBucketWithQuantum(time.Second, int64(tpm/60), int64(tpm/60))
	}
}

func InitEmbeddingLimiters(rpm, tpm int) {
	if rpm > 0 {
		EmbeddingRequestsLimiter = ratelimit.NewBucketWithQuantum(time.Second, int64(rpm/60), int64(rpm/60))
	}
	if tpm > 0 {
		EmbeddingTokensLimiter = ratelimit.NewBucketWithQuantum(time.Second, int64(tpm/60), int64(tpm/60))
	}
}

type RateLimiter interface {
	Take(count int64) time.Duration
}

type Unlimited struct{}

func (n *Unlimited) Take(count int64) time.Duration {
	return 0
}
