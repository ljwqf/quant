package risk

import "time"

// testClock 将风控引擎时钟固定在一个远离所有结算熔断窗口
// (xx:55-xx:05) 的正午时刻，避免测试依赖运行时的墙钟时间。
func testClock() EngineOption {
	fixed := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return WithClock(func() time.Time { return fixed })
}
