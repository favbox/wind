package retry

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Apply(t *testing.T) {
	var delayPolicyFunc DelayPolicyFunc = func(attempts uint, err error, retryConfig *Config) time.Duration {
		return time.Second
	}

	var opts []Option
	opts = append(opts,
		WithMaxAttemptTimes(100),
		WithInitDelay(time.Second),
		WithMaxDelay(time.Second),
		WithDelayPolicy(delayPolicyFunc),
		WithMaxJitter(time.Second),
	)

	conf := Config{}
	conf.Apply(opts)

	assert.Equal(t, uint(100), conf.MaxAttemptTimes)
	assert.Equal(t, time.Second, conf.Delay)
	assert.Equal(t, time.Second, conf.MaxDelay)
	assert.Equal(t, time.Second, Delay(0, nil, &conf))
	assert.Equal(t, time.Second, conf.MaxJitter)
}

func TestRetryPolicy(t *testing.T) {
	dur := DefaultDelayPolicy(0, nil, nil)
	assert.Equal(t, 0*time.Millisecond, dur)

	conf := Config{Delay: time.Second}
	dur = FixedDelayPolicy(0, nil, &conf)
	assert.Equal(t, time.Second, dur)

	dur = RandomDelayPolicy(0, nil, &conf)
	assert.Equal(t, 0*time.Millisecond, dur)
	conf.MaxJitter = 1 * time.Second
	dur = RandomDelayPolicy(0, nil, &conf)
	assert.NotEqual(t, 1*time.Second, dur)

	dur = BackoffDelayPolicy(0, nil, &conf)
	assert.Equal(t, time.Second*1, dur)
	conf.Delay = time.Duration(-1)
	dur = BackoffDelayPolicy(0, nil, &conf)
	assert.Equal(t, time.Second*0, dur)
	conf.Delay = time.Duration(1)
	dur = BackoffDelayPolicy(63, nil, &conf)
	durExp := conf.Delay << 62
	assert.Equal(t, durExp, dur)

	dur = Delay(0, nil, &conf)
	assert.Equal(t, 0*time.Millisecond, dur)
	delayPolicyFunc := func(attempts uint, err error, retryConfig *Config) time.Duration {
		return time.Second
	}
	conf.DelayPolicy = delayPolicyFunc
	conf.MaxDelay = time.Second / 2
	dur = Delay(0, nil, &conf)
	assert.Equal(t, conf.MaxDelay, dur)

	delayPolicyFunc2 := func(attempts uint, err error, retryConfig *Config) time.Duration {
		return time.Duration(math.MaxInt64)
	}
	delayFunc := CombineDelay(delayPolicyFunc2, delayPolicyFunc)
	dur = delayFunc(0, nil, &conf)
	assert.Equal(t, time.Duration(math.MaxInt64), dur)
}
