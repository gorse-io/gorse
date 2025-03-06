package parallel

import (
	"sync"
	"time"

	"github.com/juju/ratelimit"
)

var (
	ChatCompletionBackOff                     = NewBackOff()
	ChatCompletionRequestsLimiter RateLimiter = &Unlimited{}
	ChatCompletionTokensLimiter   RateLimiter = &Unlimited{}
	EmbeddingBackOff                          = NewBackOff()
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

type BackOff struct {
	mu     sync.Mutex
	factor int
}

func NewBackOff() *BackOff {
	return &BackOff{
		factor: 1,
	}
}

func (b *BackOff) Factor() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.factor
}

func (b *BackOff) BackOff() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.factor *= 2
}

func (b *BackOff) Recover() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.factor = max(1, b.factor-1)
}
