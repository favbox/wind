package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/favbox/wind/app/client/retry"
	"github.com/favbox/wind/common/config"
	"github.com/stretchr/testify/assert"
)

func TestClientOptions(t *testing.T) {
	opt := config.NewClientOptions([]config.ClientOption{
		WithDialTimeout(100 * time.Millisecond),
		WithMaxConnsPerHost(128),
		WithMaxIdleConnDuration(5 * time.Second),
		WithMaxConnDuration(10 * time.Second),
		WithMaxConnWaitTimeout(5 * time.Second),
		WithKeepAlive(false),
		WithClientReadTimeout(1 * time.Second),
		WithResponseBodyStream(true),
		WithRetryConfig(
			retry.WithMaxAttemptTimes(2),
			retry.WithInitDelay(100*time.Millisecond),
			retry.WithMaxDelay(5*time.Second),
			retry.WithMaxJitter(1*time.Second),
			retry.WithDelayPolicy(retry.CombineDelay(retry.FixedDelayPolicy, retry.BackoffDelayPolicy, retry.RandomDelayPolicy)),
		),
		WithWriteTimeout(time.Second),
		WithConnStateObserve(nil, time.Second),
	})
	assert.Equal(t, 100*time.Millisecond, opt.DialTimeout)
	assert.Equal(t, 128, opt.MaxConnsPerHost)
	assert.Equal(t, 5*time.Second, opt.MaxIdleConnDuration)
	assert.Equal(t, 10*time.Second, opt.MaxConnDuration)
	assert.Equal(t, 5*time.Second, opt.MaxConnWaitTimeout)
	assert.Equal(t, false, opt.KeepAlive)
	assert.Equal(t, 1*time.Second, opt.ReadTimeout)
	assert.Equal(t, 1*time.Second, opt.WriteTimeout)
	assert.Equal(t, true, opt.ResponseBodyStream)
	assert.Equal(t, uint(2), opt.RetryConfig.MaxAttemptTimes)
	assert.Equal(t, 100*time.Millisecond, opt.RetryConfig.Delay)
	assert.Equal(t, 5*time.Second, opt.RetryConfig.MaxDelay)
	assert.Equal(t, 1*time.Second, opt.RetryConfig.MaxJitter)
	assert.Equal(t, 1*time.Second, opt.ObservationInterval)
	assert.Equal(t, fmt.Sprint(retry.CombineDelay(retry.FixedDelayPolicy, retry.BackoffDelayPolicy, retry.RandomDelayPolicy)), fmt.Sprint(opt.RetryConfig.DelayPolicy))
}
