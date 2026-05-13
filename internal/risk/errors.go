package risk

import "errors"

var (
	ErrDailyLossExceeded       = errors.New("日亏损超过限制")
	ErrTradeLimitExceeded      = errors.New("交易次数超过限制")
	ErrPositionLimitExceeded   = errors.New("持仓超过限制")
	ErrInvalidOrder            = errors.New("无效订单")
	ErrLiquidityInsufficient   = errors.New("流动性不足")
	ErrCircuitBreakerOpen      = errors.New("熔断器已打开")
	ErrKillSwitchTriggered     = errors.New("Kill Switch已触发")
	ErrStrategyPaused          = errors.New("策略已暂停")
	ErrRiskBudgetExceeded      = errors.New("风险预算超限")
	ErrCorrelationTooHigh      = errors.New("相关性过高")
	ErrVolatilityTooHigh       = errors.New("波动率过高")
	ErrInvalidSignal           = errors.New("无效信号")
	ErrInsufficientMargin      = errors.New("保证金不足")
	ErrMarketClosed            = errors.New("市场已关闭")
	ErrPriceDeviationTooHigh   = errors.New("价格偏差过大")
	ErrMaxDrawdownExceeded     = errors.New("最大回撤超过限制")
	ErrSingleTradeRiskExceeded = errors.New("单笔交易风险超过限制")
	ErrSymbolExposureExceeded  = errors.New("品种风险敞口超过限制")
	ErrTotalExposureExceeded   = errors.New("总风险敞口超过限制")
)
