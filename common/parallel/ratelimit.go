package parallel

import (
	"github.com/juju/ratelimit"
	"time"
)

var (
	tpmLimiter RateLimiter = &NoRateLimiter{}
	rpmLimiter RateLimiter = &NoRateLimiter{}
)

func RPMLimiter() RateLimiter {
	return rpmLimiter
}

func TPMLimiter() RateLimiter {
	return tpmLimiter
}

func InitLimiters(tpm, rpm int) {
	if tpm > 0 {
		tpmLimiter = ratelimit.NewBucketWithQuantum(time.Minute, int64(tpm), int64(tpm))
	}
	if rpm > 0 {
		rpmLimiter = ratelimit.NewBucketWithQuantum(time.Minute, int64(rpm), int64(rpm))
	}
}

type RateLimiter interface {
	Take(count int64) time.Duration
}

type NoRateLimiter struct{}

func (n *NoRateLimiter) Take(count int64) time.Duration {
	return 0
}
