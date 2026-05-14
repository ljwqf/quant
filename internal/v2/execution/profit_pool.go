package execution

import (
	"time"
)

type ProfitPoolConfig struct {
	BaseRiskPercent   float64
	ProfitRiskPercent float64
	BaseCapital       float64
	WithdrawalRule    ProfitWithdrawalRule
}

type ProfitWithdrawalRule struct {
	WithdrawalThreshold float64
	WithdrawalPercent   float64
}

func DefaultProfitPoolConfig() ProfitPoolConfig {
	return ProfitPoolConfig{
		BaseRiskPercent:   0.01,
		ProfitRiskPercent: 0.02,
		BaseCapital:       10000,
		WithdrawalRule: ProfitWithdrawalRule{
			WithdrawalThreshold: 0.5,
			WithdrawalPercent:   0.3,
		},
	}
}

type ProfitPool struct {
	config          ProfitPoolConfig
	baseCapital     float64
	profitPool      float64
	withdrawnProfit float64
	realizedPnL     float64
	activeRisk      float64
	profitRiskUsed  float64
	createdAt       time.Time
}

type RiskAllocation struct {
	BaseRisk    float64 `json:"base_risk"`
	ExtraRisk   float64 `json:"extra_risk"`
	TotalRisk   float64 `json:"total_risk"`
	Source      string  `json:"source"`
	SignalScore float64 `json:"signal_score"`
}

func NewProfitPool(config ProfitPoolConfig) *ProfitPool {
	if config.BaseRiskPercent == 0 {
		config.BaseRiskPercent = DefaultProfitPoolConfig().BaseRiskPercent
	}
	if config.ProfitRiskPercent == 0 {
		config.ProfitRiskPercent = DefaultProfitPoolConfig().ProfitRiskPercent
	}
	if config.BaseCapital == 0 {
		config.BaseCapital = DefaultProfitPoolConfig().BaseCapital
	}
	return &ProfitPool{
		config:      config,
		baseCapital: config.BaseCapital,
		createdAt:   time.Now(),
	}
}

func (p *ProfitPool) AllocateRisk(score float64) RiskAllocation {
	baseRisk := p.baseCapital * p.config.BaseRiskPercent
	extraRisk := 0.0

	if p.profitPool > 0 && score >= 0.75 {
		extraRisk = min(p.profitPool*p.config.ProfitRiskPercent, score*baseRisk)
	}

	return RiskAllocation{
		BaseRisk:    baseRisk,
		ExtraRisk:   extraRisk,
		TotalRisk:   baseRisk + extraRisk,
		Source:      "profit_pool",
		SignalScore: score,
	}
}

func (p *ProfitPool) RecordRealizedPnL(pnl float64) {
	p.realizedPnL += pnl
	if pnl > 0 {
		p.profitPool += pnl
		p.applyWithdrawalRule()
	} else {
		lossFromProfit := min(-pnl, p.profitPool)
		p.profitPool -= lossFromProfit
	}
}

func (p *ProfitPool) applyWithdrawalRule() {
	poolRatio := p.profitPool / p.baseCapital
	if poolRatio >= p.config.WithdrawalRule.WithdrawalThreshold {
		withdrawal := p.profitPool * p.config.WithdrawalRule.WithdrawalPercent
		p.profitPool -= withdrawal
		p.withdrawnProfit += withdrawal
	}
}

func (p *ProfitPool) BaseCapital() float64 { return p.baseCapital }
func (p *ProfitPool) ProfitPool() float64  { return p.profitPool }
func (p *ProfitPool) Withdrawn() float64   { return p.withdrawnProfit }
func (p *ProfitPool) RealizedPnL() float64 { return p.realizedPnL }
func (p *ProfitPool) NetProfit() float64   { return p.profitPool + p.withdrawnProfit }

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
