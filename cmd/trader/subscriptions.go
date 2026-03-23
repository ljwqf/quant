package main

import (
	"errors"
	"fmt"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

func dispatchStrategyResult(result *strategy.StrategyResult, executeSignal func(*types.Signal), logError func(error)) {
	if result == nil {
		return
	}

	for _, signal := range result.Signals {
		if signal == nil {
			continue
		}
		executeSignal(signal)
	}

	for _, err := range result.Errors {
		if err == nil {
			continue
		}
		logError(err)
	}
}

func subscribeStrategyMarketData(ex exchange.Exchange, symbol, barInterval string, strategyEngine *strategy.Engine, executeSignal func(*types.Signal)) error {
	var errs []error

	if err := ex.SubscribeTicker(symbol, func(tick *types.Tick) {
		dispatchStrategyResult(strategyEngine.OnTick(tick), executeSignal, func(err error) {
			logger.Error("策略OnTick错误", zap.Error(err))
		})
	}); err != nil {
		errs = append(errs, fmt.Errorf("订阅Tick失败: %w", err))
	}

	if err := ex.SubscribeBar(symbol, barInterval, func(bar *types.Bar) {
		dispatchStrategyResult(strategyEngine.OnBar(bar), executeSignal, func(err error) {
			logger.Error("策略OnBar错误", zap.Error(err))
		})
	}); err != nil {
		errs = append(errs, fmt.Errorf("订阅Bar失败: %w", err))
	}

	if err := ex.SubscribeOrderBook(symbol, func(orderBook *types.OrderBook) {
		dispatchStrategyResult(strategyEngine.OnOrderBook(orderBook), executeSignal, func(err error) {
			logger.Error("策略OnOrderBook错误", zap.Error(err))
		})
	}); err != nil {
		errs = append(errs, fmt.Errorf("订阅OrderBook失败: %w", err))
	}

	return errors.Join(errs...)
}
