package strategy

import (
	"math"
	"sync"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// TestBuySellStrategy 测试策略：用 1% 资金买入，1 分钟后卖出
// 用于验证真实交易能力（下单、成交、持仓、平仓全链路）
type TestBuySellStrategy struct {
	mu              sync.Mutex
	initialized     bool
	symbol          string
	position        *testPosition
	firstTickTime   time.Time
	tickCount       int
	buyTriggered    bool
	sellTriggered   bool
	signalCallback  func(*types.Signal)
	currentPrice    float64
	accountBalance  float64
}

type testPosition struct {
	symbol     string
	side       types.OrderSide
	entryPrice float64
	size       float64
	openTime   time.Time
}

func NewTestBuySellStrategy() *TestBuySellStrategy {
	return &TestBuySellStrategy{
		symbol: "BTC-USDT-SWAP",
	}
}

func (s *TestBuySellStrategy) Name() string {
	return "TestBuySellStrategy"
}

func (s *TestBuySellStrategy) Init(params map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if symbol, ok := params["symbol"].(string); ok && symbol != "" {
		s.symbol = symbol
	}

	s.initialized = true
	logger.Info("TestBuySellStrategy 初始化完成",
		zap.String("symbol", s.symbol),
	)
	return nil
}

func (s *TestBuySellStrategy) SetSignalCallback(cb func(*types.Signal)) {
	s.signalCallback = cb
}

func (s *TestBuySellStrategy) SetAccountBalance(balance float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountBalance = balance
}

func (s *TestBuySellStrategy) OnTick(tick *types.Tick) (*types.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil, nil
	}

	s.currentPrice = tick.Price
	s.tickCount++

	if s.firstTickTime.IsZero() {
		s.firstTickTime = time.Now()
	}

	// 首次收到行情且未持仓 -> 买入
	if !s.buyTriggered && s.position == nil {
		s.buyTriggered = true
		// 用 1% 资金计算买入数量
		buyAmount := s.accountBalance * 0.01
		if buyAmount <= 0 {
			// 如果还没设置账户余额，用固定数量测试
			buyAmount = tick.Price * 0.001 // 约 0.1% 的 1 BTC
		}
		// BTC-USDT-SWAP 合约面值: 1 张 = 0.001 BTC
		// 将 USDT 金额转换为合约张数（必须为整数）
		contracts := buyAmount / tick.Price / 0.001
		contracts = math.Round(contracts)
		if contracts < 1 {
			contracts = 1
		}

		logger.Info("TestBuySell: 触发买入",
			zap.String("symbol", s.symbol),
			zap.Float64("price", tick.Price),
			zap.Float64("buy_amount", buyAmount),
			zap.Float64("contracts", contracts),
			zap.Float64("account_balance", s.accountBalance),
		)

		s.position = &testPosition{
			symbol:     s.symbol,
			side:       types.OrderSideBuy,
			entryPrice: tick.Price,
			size:       contracts,
			openTime:   time.Now(),
		}

		signal := &types.Signal{
			Strategy:   s.Name(),
			Symbol:     s.symbol,
			Type:       types.SignalTypeBuy,
			Price:      tick.Price,
			Quantity:   contracts,
			Confidence: 1.0,
			Timestamp:  time.Now(),
		}

		return signal, nil
	}

	// 已持仓且超过 1 分钟 -> 卖出
	if s.position != nil && !s.sellTriggered && time.Since(s.position.openTime) >= 1*time.Minute {
		s.sellTriggered = true
		holdDuration := time.Since(s.position.openTime)
		pnlPercent := 0.0
		if s.position.entryPrice > 0 {
			pnlPercent = (tick.Price - s.position.entryPrice) / s.position.entryPrice * 100
		}

		logger.Info("TestBuySell: 触发卖出",
			zap.String("symbol", s.symbol),
			zap.Float64("entry_price", s.position.entryPrice),
			zap.Float64("current_price", tick.Price),
			zap.Float64("size", s.position.size),
			zap.Duration("hold_duration", holdDuration),
			zap.Float64("pnl_percent", pnlPercent),
		)

		signal := &types.Signal{
			Strategy:   s.Name(),
			Symbol:     s.symbol,
			Type:       types.SignalTypeExit,
			Price:      tick.Price,
			Confidence: 1.0,
			Timestamp:  time.Now(),
		}

		return signal, nil
	}

	return nil, nil
}

func (s *TestBuySellStrategy) OnBar(bar *types.Bar) (*types.Signal, error) {
	return nil, nil
}

func (s *TestBuySellStrategy) OnOrderBook(orderBook *types.OrderBook) (*types.Signal, error) {
	return nil, nil
}

func (s *TestBuySellStrategy) GetParams() map[string]interface{} {
	return map[string]interface{}{
		"symbol": s.symbol,
	}
}

func (s *TestBuySellStrategy) SetParams(params map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if symbol, ok := params["symbol"].(string); ok && symbol != "" {
		s.symbol = symbol
	}
}

func (s *TestBuySellStrategy) GetMetrics() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics := map[string]interface{}{
		"initialized":   s.initialized,
		"tick_count":    s.tickCount,
		"buy_triggered": s.buyTriggered,
		"sell_triggered": s.sellTriggered,
		"current_price": s.currentPrice,
	}

	if s.position != nil {
		metrics["position_size"] = s.position.size
		metrics["entry_price"] = s.position.entryPrice
		metrics["open_time"] = s.position.openTime
	}

	return metrics
}

func (s *TestBuySellStrategy) OnPositionFilled(symbol string, side types.OrderSide, entryPrice, size float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.position != nil && s.position.symbol == symbol {
		s.position.entryPrice = entryPrice
		s.position.size = size
	}

	logger.Info("TestBuySell: 订单成交确认",
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("size", size),
	)
}

func (s *TestBuySellStrategy) OnPositionReduced(symbol string, exitPrice, pnl, remainingSize float64) {
	logger.Info("TestBuySell: 部分平仓",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
		zap.Float64("remaining_size", remainingSize),
	)
}

func (s *TestBuySellStrategy) OnPositionClosed(symbol string, exitPrice, pnl float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Info("TestBuySell: 平仓完成",
		zap.String("symbol", symbol),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
	)

	s.position = nil
}

func (s *TestBuySellStrategy) ConfirmRebalanceEntry(request *RebalanceRequest) (*RebalanceDecision, error) {
	return &RebalanceDecision{
		Approved: true,
	}, nil
}

// Stop 停止策略
func (s *TestBuySellStrategy) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger.Info("TestBuySellStrategy 已停止",
		zap.Int("tick_count", s.tickCount),
		zap.Bool("buy_triggered", s.buyTriggered),
		zap.Bool("sell_triggered", s.sellTriggered),
	)
}
