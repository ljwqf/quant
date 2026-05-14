package execution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/v2/events"
)

type PaperPosition struct {
	Symbol         string      `json:"symbol"`
	StrategyID     string      `json:"strategy_id"`
	Side           events.Side `json:"side"`
	EntryPrice     float64     `json:"entry_price"`
	Size           float64     `json:"size"`
	EntryTime      time.Time   `json:"entry_time"`
	StructuralStop float64     `json:"structural_stop"`
	UnrealizedPnL  float64     `json:"unrealized_pnl"`
	ExitPrice      float64     `json:"exit_price,omitempty"`
	ExitTime       time.Time   `json:"exit_time,omitempty"`
	RealizedPnL    float64     `json:"realized_pnl,omitempty"`
	StopReason     string      `json:"stop_reason,omitempty"`
}

type PaperExecutorConfig struct {
	LogDir string
}

func DefaultPaperExecutorConfig() PaperExecutorConfig {
	return PaperExecutorConfig{LogDir: "logs/paper_trades"}
}

type PaperExecutor struct {
	config     PaperExecutorConfig
	pool       *ProfitPool
	riskGuard  *RiskGuard
	positions  map[string]*PaperPosition
	mu         sync.Mutex
	tradeLog   *os.File
	encoder    *json.Encoder
	tradeCount int
	winCount   int
	lossCount  int
	totalPnL   float64
}

func NewPaperExecutor(config PaperExecutorConfig, pool *ProfitPool, riskGuard *RiskGuard) (*PaperExecutor, error) {
	if config.LogDir == "" {
		config.LogDir = DefaultPaperExecutorConfig().LogDir
	}
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, err
	}

	tradeLogPath := filepath.Join(config.LogDir, "paper_trades.json")
	tradeLog, err := os.OpenFile(tradeLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &PaperExecutor{
		config:    config,
		pool:      pool,
		riskGuard: riskGuard,
		positions: make(map[string]*PaperPosition),
		tradeLog:  tradeLog,
		encoder:   json.NewEncoder(tradeLog),
	}, nil
}

func (e *PaperExecutor) OpenPosition(intent events.SignalIntent) (*PaperPosition, RiskCheckResult) {
	riskCheck := e.riskGuard.CheckEntry(intent, e.pool.BaseCapital())
	if !riskCheck.Allowed {
		return nil, riskCheck
	}

	allocation := e.pool.AllocateRisk(intent.Score)
	positionSize := allocation.TotalRisk / intent.ExpectedEntry

	pos := &PaperPosition{
		Symbol:         intent.Symbol,
		StrategyID:     intent.StrategyID,
		Side:           intent.Side,
		EntryPrice:     intent.ExpectedEntry,
		Size:           positionSize,
		EntryTime:      intent.Timestamp,
		StructuralStop: intent.StructuralStop,
	}

	e.mu.Lock()
	e.positions[intent.Symbol+"_"+intent.StrategyID] = pos
	e.mu.Unlock()

	e.logTrade("paper_open", pos, intent)

	return pos, riskCheck
}

func (e *PaperExecutor) UpdatePrice(symbol string, currentPrice float64, now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for key, pos := range e.positions {
		if pos.Symbol != symbol || pos.ExitTime.After(time.Time{}) {
			continue
		}

		pos.UnrealizedPnL = e.calculatePnL(pos, currentPrice)

		stopRule := StructuralStopRule{
			StrategyID: pos.StrategyID,
			Side:       pos.Side,
			WallPrice:  pos.StructuralStop,
			BufferBps:  20,
		}
		shouldStop, reason := e.riskGuard.CheckStructuralStop(pos.Side, currentPrice, stopRule)
		if shouldStop {
			e.closePosition(key, pos, currentPrice, now, reason)
		}

		hardStopBuffer := e.riskGuard.hardRisk.MaxLossPerTradeBps / 10000
		if pos.Side == events.SideBuy && currentPrice <= pos.EntryPrice*(1-hardStopBuffer) {
			e.closePosition(key, pos, currentPrice, now, "hard_stop_loss")
		} else if pos.Side == events.SideSell && currentPrice >= pos.EntryPrice*(1+hardStopBuffer) {
			e.closePosition(key, pos, currentPrice, now, "hard_stop_loss")
		}
	}
}

func (e *PaperExecutor) ClosePosition(symbol, strategyID string, exitPrice float64, now time.Time, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := symbol + "_" + strategyID
	pos, ok := e.positions[key]
	if !ok || pos.ExitTime.After(time.Time{}) {
		return
	}
	e.closePosition(key, pos, exitPrice, now, reason)
}

func (e *PaperExecutor) closePosition(key string, pos *PaperPosition, exitPrice float64, now time.Time, reason string) {
	pos.ExitPrice = exitPrice
	pos.ExitTime = now
	pos.RealizedPnL = e.calculatePnL(pos, exitPrice)
	pos.StopReason = reason

	e.pool.RecordRealizedPnL(pos.RealizedPnL)
	e.tradeCount++

	if pos.RealizedPnL > 0 {
		e.winCount++
		e.riskGuard.RecordWin(pos.StrategyID)
	} else {
		e.lossCount++
		pnlBps := pos.RealizedPnL / (pos.EntryPrice * pos.Size) * 10000
		e.riskGuard.RecordLoss(pos.StrategyID, pnlBps)
	}

	e.totalPnL += pos.RealizedPnL
	e.logTrade("paper_close", pos, events.SignalIntent{})
}

func (e *PaperExecutor) calculatePnL(pos *PaperPosition, price float64) float64 {
	if pos.Side == events.SideBuy {
		return (price - pos.EntryPrice) * pos.Size
	}
	return (pos.EntryPrice - price) * pos.Size
}

func (e *PaperExecutor) logTrade(eventType string, pos *PaperPosition, intent events.SignalIntent) {
	record := map[string]interface{}{
		"event_type":      eventType,
		"timestamp":       pos.EntryTime,
		"symbol":          pos.Symbol,
		"strategy_id":     pos.StrategyID,
		"side":            pos.Side,
		"entry_price":     pos.EntryPrice,
		"size":            pos.Size,
		"structural_stop": pos.StructuralStop,
	}
	if pos.ExitPrice > 0 {
		record["exit_price"] = pos.ExitPrice
		record["realized_pnl"] = pos.RealizedPnL
		record["stop_reason"] = pos.StopReason
	}
	e.encoder.Encode(record)
}

func (e *PaperExecutor) Stats() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	winRate := 0.0
	if e.tradeCount > 0 {
		winRate = float64(e.winCount) / float64(e.tradeCount)
	}
	return map[string]interface{}{
		"trade_count": e.tradeCount,
		"win_count":   e.winCount,
		"loss_count":  e.lossCount,
		"win_rate":    winRate,
		"total_pnl":   e.totalPnL,
		"profit_pool": e.pool.ProfitPool(),
		"read_only":   e.riskGuard.IsReadOnly(),
	}
}

func (e *PaperExecutor) Close() error {
	return e.tradeLog.Close()
}
