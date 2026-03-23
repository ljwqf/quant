package backtest

import (
	"fmt"
	"math"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// Strategy 回测策略接口
type Strategy interface {
	Name() string
	Init(params map[string]interface{}) error
	OnBar(bar *types.Bar) (*types.Signal, error)
	GetParameters() map[string]interface{}
}

// Result 回测结果
type Result struct {
	TotalTrades       int     `json:"total_trades"`
	WinTrades         int     `json:"win_trades"`
	LossTrades        int     `json:"loss_trades"`
	WinRate           float64 `json:"win_rate"`
	TotalPnL          float64 `json:"total_pnl"`
	MaxDrawdown       float64 `json:"max_drawdown"`
	SharpeRatio       float64 `json:"sharpe_ratio"`
	AverageWin        float64 `json:"average_win"`
	AverageLoss       float64 `json:"average_loss"`
	ProfitFactor      float64 `json:"profit_factor"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
	InitialBalance    float64 `json:"initial_balance"`
	FinalBalance      float64 `json:"final_balance"`
	EquityCurve       []EquityPoint `json:"equity_curve"`
	Trades            []Trade `json:"trades"`
}

// EquityPoint 权益曲线点
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Equity    float64   `json:"equity"`
}

// Trade 回测交易记录
type Trade struct {
	ID        string      `json:"id"`
	Symbol    string      `json:"symbol"`
	Side      types.OrderSide `json:"side"`
	EntryPrice float64     `json:"entry_price"`
	ExitPrice  float64     `json:"exit_price"`
	Quantity   float64     `json:"quantity"`
	PnL        float64     `json:"pnl"`
	PnLPercent float64     `json:"pnl_percent"`
	EntryTime  time.Time   `json:"entry_time"`
	ExitTime   time.Time   `json:"exit_time"`
	HoldingPeriod time.Duration `json:"holding_period"`
}

// Engine 回测引擎
type Engine struct {
	strategy    Strategy
	dataManager *DataManager
	simulator   *Simulator
	result      *Result
	initialBalance float64
	startTime   time.Time
	endTime     time.Time
}

// NewEngine 创建回测引擎
func NewEngine(strategy Strategy, initialBalance float64) *Engine {
	return &Engine{
		strategy:    strategy,
		dataManager: NewDataManager(),
		simulator:   NewSimulator(initialBalance),
		result:      &Result{
			EquityCurve:   make([]EquityPoint, 0),
			Trades:        make([]Trade, 0),
			InitialBalance: initialBalance,
		},
		initialBalance: initialBalance,
	}
}

// AddData 添加历史数据
func (e *Engine) AddData(symbol string, bars []*types.Bar) error {
	return e.dataManager.AddData(symbol, bars)
}

// LoadDataFromFile 从文件加载历史数据
func (e *Engine) LoadDataFromFile(filePath string, symbol string) error {
	return e.dataManager.LoadFromFile(filePath, symbol)
}

// Run 运行回测
func (e *Engine) Run() error {
	logger.Info("开始回测",
		zap.String("strategy", e.strategy.Name()),
		zap.Float64("initial_balance", e.initialBalance),
	)

	// 按时间顺序处理K线数据
	data, err := e.dataManager.GetSortedData()
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return fmt.Errorf("没有数据可供回测")
	}

	e.startTime = data[0].Timestamp
	e.endTime = data[len(data)-1].Timestamp
	e.result.StartTime = e.startTime
	e.result.EndTime = e.endTime

	// 处理每根K线
	for i, bar := range data {
		// 执行策略
		signal, err := e.strategy.OnBar(bar)
		if err != nil {
			logger.Error("策略执行错误", zap.Error(err))
			continue
		}

		// 处理交易信号
		if signal != nil && (signal.Type == types.SignalTypeBuy || signal.Type == types.SignalTypeSell) {
			trade, err := e.simulator.ExecuteTrade(signal, bar)
			if err != nil {
				logger.Error("执行交易错误", zap.Error(err))
				continue
			}

			if trade != nil {
				e.result.Trades = append(e.result.Trades, *trade)
			}
		}

		// 记录权益曲线
		if i%10 == 0 { // 每10根K线记录一次
			equity := e.simulator.GetEquity()
			e.result.EquityCurve = append(e.result.EquityCurve, EquityPoint{
				Timestamp: bar.Timestamp,
				Equity:    equity,
			})
		}
	}

	// 平仓所有持仓
	closedTrades, err := e.simulator.CloseAllPositions(data[len(data)-1])
	if err != nil {
		logger.Error("平仓错误", zap.Error(err))
	}
	e.result.Trades = append(e.result.Trades, closedTrades...)

	// 计算回测结果
	e.calculateResults()

	// 生成报告
	e.generateReport()

	logger.Info("回测完成",
		zap.Int("total_trades", e.result.TotalTrades),
		zap.Float64("total_pnl", e.result.TotalPnL),
		zap.Float64("win_rate", e.result.WinRate),
		zap.Float64("max_drawdown", e.result.MaxDrawdown),
		zap.Float64("sharpe_ratio", e.result.SharpeRatio),
	)

	return nil
}

// calculateResults 计算回测结果
func (e *Engine) calculateResults() {
	totalTrades := len(e.result.Trades)
	e.result.TotalTrades = totalTrades

	if totalTrades == 0 {
		e.result.FinalBalance = e.initialBalance
		e.result.TotalPnL = 0
		return
	}

	totalWin := 0
	totalLoss := 0
	totalWinAmount := 0.0
	totalLossAmount := 0.0
	maxDrawdown := 0.0
	peakEquity := e.initialBalance

	for _, trade := range e.result.Trades {
		if trade.PnL > 0 {
			totalWin++
			totalWinAmount += trade.PnL
		} else if trade.PnL < 0 {
			totalLoss++
			totalLossAmount += trade.PnL
		}
	}

	e.result.WinTrades = totalWin
	e.result.LossTrades = totalLoss

	if totalTrades > 0 {
		e.result.WinRate = float64(totalWin) / float64(totalTrades)
	}

	// 计算总盈亏
	e.result.TotalPnL = totalWinAmount + totalLossAmount
	e.result.FinalBalance = e.initialBalance + e.result.TotalPnL

	// 计算平均盈亏
	if totalWin > 0 {
		e.result.AverageWin = totalWinAmount / float64(totalWin)
	}
	if totalLoss > 0 {
		e.result.AverageLoss = totalLossAmount / float64(totalLoss)
	}

	// 计算盈利因子
	if totalLossAmount != 0 {
		e.result.ProfitFactor = -totalWinAmount / totalLossAmount
	}

	// 计算最大回撤
	currentEquity := e.initialBalance
	for _, point := range e.result.EquityCurve {
		currentEquity = point.Equity
		if currentEquity > peakEquity {
			peakEquity = currentEquity
		} else {
			drawdown := (peakEquity - currentEquity) / peakEquity
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}
	e.result.MaxDrawdown = maxDrawdown

	// 计算夏普比率
	// 夏普比率 = (平均收益率 - 无风险利率) / 收益率标准差
	if len(e.result.EquityCurve) > 1 {
		// 计算每日收益率
		dailyReturns := make([]float64, 0, len(e.result.EquityCurve)-1)
		for i := 1; i < len(e.result.EquityCurve); i++ {
			prevEquity := e.result.EquityCurve[i-1].Equity
			currEquity := e.result.EquityCurve[i].Equity
			if prevEquity > 0 {
				dailyReturn := (currEquity - prevEquity) / prevEquity
				dailyReturns = append(dailyReturns, dailyReturn)
			}
		}

		if len(dailyReturns) > 0 {
			// 计算平均收益率
			var sumReturns float64
			for _, r := range dailyReturns {
				sumReturns += r
			}
			averageReturn := sumReturns / float64(len(dailyReturns))

			// 计算收益率标准差
			var sumSquaredDiff float64
			for _, r := range dailyReturns {
				diff := r - averageReturn
				sumSquaredDiff += diff * diff
			}
			stdDev := math.Sqrt(sumSquaredDiff / float64(len(dailyReturns)))

			// 假设无风险利率为0
			if stdDev > 0 {
				e.result.SharpeRatio = averageReturn / stdDev * math.Sqrt(252) // 年化
			}
		}
	}
}

// generateReport 生成回测报告
func (e *Engine) generateReport() {
	// 这里可以生成详细的回测报告
	// 包括交易记录、权益曲线、绩效指标等
	logger.Info("生成回测报告",
		zap.String("strategy", e.strategy.Name()),
		zap.Float64("total_pnl", e.result.TotalPnL),
		zap.Float64("win_rate", e.result.WinRate),
		zap.Float64("max_drawdown", e.result.MaxDrawdown),
	)
}

// GetResult 获取回测结果
func (e *Engine) GetResult() *Result {
	return e.result
}
