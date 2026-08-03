package execution

import "time"

// execTestClock 返回固定时钟，供 risk.WithClock 使用，
// 避免测试在结算熔断窗口 (如 23:55-00:05) 内运行时失败。
func execTestClock() func() time.Time {
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return fixed }
}
