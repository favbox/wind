package retry

import (
	"math"
	"time"

	"github.com/bytedance/gopkg/lang/fastrand"
)

// DelayPolicyFunc 定义延迟策略函数，返回重试的延迟时间。
type DelayPolicyFunc func(attempts uint, err error, retryConfig *Config) time.Duration

// DefaultDelayPolicy 是一个零延迟策略的 DelayPolicyFunc。
// 在所有的迭代中都保持零延迟。
func DefaultDelayPolicy(_ uint, _ error, _ *Config) time.Duration {
	return 0 * time.Millisecond
}

// FixedDelayPolicy 是一个固定延迟策略的 DelayPolicyFunc。
// 在所有的迭代中都保持相同的延迟时间。
func FixedDelayPolicy(_ uint, _ error, retryConfig *Config) time.Duration {
	return retryConfig.Delay
}

// RandomDelayPolicy 是一个随机延迟策略的 DelayPolicyFunc。
// 它选取一个最大为 Config.MaxJitter 的随机延迟，若 Config.MaxJitter <= 0，则进行零延迟。
func RandomDelayPolicy(_ uint, _ error, retryConfig *Config) time.Duration {
	if retryConfig.MaxJitter <= 0 {
		return 0 * time.Millisecond
	}
	return time.Duration(fastrand.Int63n(int64(retryConfig.MaxJitter)))
}

// BackoffDelayPolicy 是一种回退延迟策略的 DelayPolicyFunc。
// 它会成倍增加连续重试之间的延迟，若 Config.Delay <= 0，则进行零延迟。
func BackoffDelayPolicy(attempts uint, _ error, retryConfig *Config) time.Duration {
	if retryConfig.Delay <= 0 {
		return 0 * time.Millisecond
	}
	// 1 << 63 会溢出有符号 int64，故 62。
	const max uint = 62
	if attempts > max {
		attempts = max
	}

	return retryConfig.Delay << attempts
}

// CombineDelay 将多个重试策略函数组合为一个并返回。
func CombineDelay(delays ...DelayPolicyFunc) DelayPolicyFunc {
	const maxInt64 = uint64(math.MaxInt64)

	return func(attempts uint, err error, retryConfig *Config) time.Duration {
		var total uint64
		for _, delay := range delays {
			total += uint64(delay(attempts, err, retryConfig))
			if total > maxInt64 {
				total = maxInt64
			}
		}

		return time.Duration(total)
	}
}

// Delay 生成指定重试配置的延迟时间。若 Config.DelayPolicy 为空，则零延迟。
func Delay(attempts uint, err error, retryConfig *Config) time.Duration {
	if retryConfig.DelayPolicy == nil {
		return 0 * time.Millisecond
	}

	delayTime := retryConfig.DelayPolicy(attempts, err, retryConfig)
	if retryConfig.MaxDelay > 0 && delayTime > retryConfig.MaxDelay {
		delayTime = retryConfig.MaxDelay
	}
	return delayTime
}
