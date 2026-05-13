package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// ReportMetrics 报告指标
type ReportMetrics struct {
	TotalReturn       float64 `json:"total_return"`
	AnnualizedReturn  float64 `json:"annualized_return"`
	Volatility        float64 `json:"volatility"`
	SharpeRatio       float64 `json:"sharpe_ratio"`
	MaxDrawdown       float64 `json:"max_drawdown"`
	MaxDrawdownPeriod time.Duration `json:"max_drawdown_period"`
	WinRate           float64 `json:"win_rate"`
	ProfitFactor      float64 `json:"profit_factor"`
	AverageWin        float64 `json:"average_win"`
	AverageLoss       float64 `json:"average_loss"`
	WinLossRatio      float64 `json:"win_loss_ratio"`
	TotalTrades       int     `json:"total_trades"`
	WinTrades         int     `json:"win_trades"`
	LossTrades        int     `json:"loss_trades"`
	AverageHoldingTime time.Duration `json:"average_holding_time"`
}

// MonthlyReturn 月度收益率
type MonthlyReturn struct {
	Year   int     `json:"year"`
	Month  int     `json:"month"`
	Return float64 `json:"return"`
}

// Report 回测报告
type Report struct {
	StrategyName    string        `json:"strategy_name"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	InitialBalance  float64       `json:"initial_balance"`
	FinalBalance    float64       `json:"final_balance"`
	Metrics         ReportMetrics `json:"metrics"`
	MonthlyReturns  []MonthlyReturn `json:"monthly_returns"`
	EquityCurve     []EquityPoint  `json:"equity_curve"`
	TopTrades       []Trade        `json:"top_trades"`
	WorstTrades     []Trade        `json:"worst_trades"`
	GeneratedAt     time.Time      `json:"generated_at"`
}

// ReportGenerator 报告生成器
type ReportGenerator struct {
	result *Result
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(result *Result) *ReportGenerator {
	return &ReportGenerator{
		result: result,
	}
}

// Generate 生成回测报告
func (rg *ReportGenerator) Generate(strategyName string) *Report {
	if rg.result == nil {
		return nil
	}

	duration := rg.result.EndTime.Sub(rg.result.StartTime)
	metrics := rg.calculateMetrics()
	monthlyReturns := rg.calculateMonthlyReturns()
	topTrades, worstTrades := rg.getTopAndWorstTrades(5)

	return &Report{
		StrategyName:   strategyName,
		StartTime:      rg.result.StartTime,
		EndTime:        rg.result.EndTime,
		Duration:       duration,
		InitialBalance: rg.result.InitialBalance,
		FinalBalance:   rg.result.FinalBalance,
		Metrics:        metrics,
		MonthlyReturns: monthlyReturns,
		EquityCurve:    rg.result.EquityCurve,
		TopTrades:      topTrades,
		WorstTrades:    worstTrades,
		GeneratedAt:    time.Now(),
	}
}

// calculateMetrics 计算详细指标
func (rg *ReportGenerator) calculateMetrics() ReportMetrics {
	metrics := ReportMetrics{
		TotalTrades:  rg.result.TotalTrades,
		WinTrades:    rg.result.WinTrades,
		LossTrades:   rg.result.LossTrades,
		WinRate:      rg.result.WinRate,
		SharpeRatio:  rg.result.SharpeRatio,
		MaxDrawdown:  rg.result.MaxDrawdown,
		AverageWin:   rg.result.AverageWin,
		AverageLoss:  rg.result.AverageLoss,
		ProfitFactor: rg.result.ProfitFactor,
	}

	if rg.result.InitialBalance > 0 {
		metrics.TotalReturn = (rg.result.FinalBalance - rg.result.InitialBalance) / rg.result.InitialBalance
	}

	duration := rg.result.EndTime.Sub(rg.result.StartTime)
	years := duration.Hours() / (24 * 365)
	if years > 0 && metrics.TotalReturn > -1 {
		metrics.AnnualizedReturn = math.Pow(1+metrics.TotalReturn, 1/years) - 1
	}

	if len(rg.result.EquityCurve) > 1 {
		metrics.Volatility = rg.calculateVolatility()
	}

	if metrics.AverageLoss != 0 {
		metrics.WinLossRatio = -metrics.AverageWin / metrics.AverageLoss
	}

	metrics.MaxDrawdown, metrics.MaxDrawdownPeriod = rg.calculateMaxDrawdownDetails()

	metrics.AverageHoldingTime = rg.calculateAverageHoldingTime()

	return metrics
}

// calculateVolatility 计算波动率
func (rg *ReportGenerator) calculateVolatility() float64 {
	if len(rg.result.EquityCurve) < 2 {
		return 0
	}

	returns := make([]float64, 0, len(rg.result.EquityCurve)-1)
	for i := 1; i < len(rg.result.EquityCurve); i++ {
		prev := rg.result.EquityCurve[i-1].Equity
		curr := rg.result.EquityCurve[i].Equity
		if prev > 0 {
			returns = append(returns, (curr-prev)/prev)
		}
	}

	if len(returns) == 0 {
		return 0
	}

	meanReturn := 0.0
	for _, r := range returns {
		meanReturn += r
	}
	meanReturn /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		variance += diff * diff
	}
	variance /= float64(len(returns))

	stdDev := math.Sqrt(variance)
	return stdDev * math.Sqrt(252)
}

// calculateMaxDrawdownDetails 计算最大回撤详情
func (rg *ReportGenerator) calculateMaxDrawdownDetails() (float64, time.Duration) {
	if len(rg.result.EquityCurve) < 2 {
		return 0, 0
	}

	maxDrawdown := 0.0
	peakEquity := rg.result.EquityCurve[0].Equity
	peakTime := rg.result.EquityCurve[0].Timestamp
	maxDrawdownStart := peakTime
	maxDrawdownEnd := peakTime

	for _, point := range rg.result.EquityCurve {
		if point.Equity > peakEquity {
			peakEquity = point.Equity
			peakTime = point.Timestamp
		} else {
			drawdown := (peakEquity - point.Equity) / peakEquity
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
				maxDrawdownStart = peakTime
				maxDrawdownEnd = point.Timestamp
			}
		}
	}

	return maxDrawdown, maxDrawdownEnd.Sub(maxDrawdownStart)
}

// calculateAverageHoldingTime 计算平均持仓时间
func (rg *ReportGenerator) calculateAverageHoldingTime() time.Duration {
	if len(rg.result.Trades) == 0 {
		return 0
	}

	totalDuration := time.Duration(0)
	for _, trade := range rg.result.Trades {
		totalDuration += trade.HoldingPeriod
	}

	return totalDuration / time.Duration(len(rg.result.Trades))
}

// calculateMonthlyReturns 计算月度收益率
func (rg *ReportGenerator) calculateMonthlyReturns() []MonthlyReturn {
	if len(rg.result.EquityCurve) < 2 {
		return nil
	}

	monthlyMap := make(map[string]float64)
	monthlyStartBalance := make(map[string]float64)

	for _, point := range rg.result.EquityCurve {
		key := fmt.Sprintf("%d-%02d", point.Timestamp.Year(), point.Timestamp.Month())
		
		if _, exists := monthlyStartBalance[key]; !exists {
			monthlyStartBalance[key] = point.Equity
		}

		monthlyMap[key] = point.Equity
	}

	monthlyReturns := make([]MonthlyReturn, 0, len(monthlyMap))
	for key, endBalance := range monthlyMap {
		startBalance := monthlyStartBalance[key]
		var year, month int
		fmt.Sscanf(key, "%d-%d", &year, &month)

		returnRate := 0.0
		if startBalance > 0 {
			returnRate = (endBalance - startBalance) / startBalance
		}

		monthlyReturns = append(monthlyReturns, MonthlyReturn{
			Year:   year,
			Month:  month,
			Return: returnRate,
		})
	}

	return monthlyReturns
}

// getTopAndWorstTrades 获取最好和最差的交易
func (rg *ReportGenerator) getTopAndWorstTrades(n int) ([]Trade, []Trade) {
	if len(rg.result.Trades) == 0 {
		return nil, nil
	}

	trades := make([]Trade, len(rg.result.Trades))
	copy(trades, rg.result.Trades)

	for i := 0; i < len(trades)-1; i++ {
		for j := i + 1; j < len(trades); j++ {
			if trades[j].PnL > trades[i].PnL {
				trades[i], trades[j] = trades[j], trades[i]
			}
		}
	}

	topCount := n
	if topCount > len(trades) {
		topCount = len(trades)
	}
	topTrades := trades[:topCount]

	worstCount := n
	if worstCount > len(trades) {
		worstCount = len(trades)
	}
	worstTrades := trades[len(trades)-worstCount:]

	return topTrades, worstTrades
}

// ToJSON 将报告转换为JSON
func (r *Report) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToString 将报告转换为可读的字符串
func (r *Report) ToString() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("╔══════════════════════════════════════════════════════════════╗\n"))
	sb.WriteString(fmt.Sprintf("║                    回测报告 - %-30s    ║\n", r.StrategyName))
	sb.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	sb.WriteString(fmt.Sprintf("║ 回测期间: %s 至 %s                              ║\n", 
		r.StartTime.Format("2006-01-02"), r.EndTime.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("║ 回测时长: %-50s ║\n", r.Duration.String()))
	sb.WriteString(fmt.Sprintf("║ 初始资金: %.2f                                              ║\n", r.InitialBalance))
	sb.WriteString(fmt.Sprintf("║ 最终资金: %.2f                                              ║\n", r.FinalBalance))
	sb.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	sb.WriteString(fmt.Sprintf("║                           绩效指标                             ║\n"))
	sb.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	sb.WriteString(fmt.Sprintf("║ 总收益率:   %8.2f%% | 年化收益率: %8.2f%%                ║\n", 
		r.Metrics.TotalReturn*100, r.Metrics.AnnualizedReturn*100))
	sb.WriteString(fmt.Sprintf("║ 夏普比率:   %8.2f   | 波动率:     %8.2f%%                ║\n", 
		r.Metrics.SharpeRatio, r.Metrics.Volatility*100))
	sb.WriteString(fmt.Sprintf("║ 最大回撤:   %8.2f%% | 回撤周期:   %s                    ║\n", 
		r.Metrics.MaxDrawdown*100, r.Metrics.MaxDrawdownPeriod.String()))
	sb.WriteString(fmt.Sprintf("║ 胜率:       %8.2f%% | 盈亏比:     %8.2f                   ║\n", 
		r.Metrics.WinRate*100, r.Metrics.WinLossRatio))
	sb.WriteString(fmt.Sprintf("║ 盈利因子:   %8.2f   | 平均持仓:   %s                    ║\n", 
		r.Metrics.ProfitFactor, r.Metrics.AverageHoldingTime.String()))
	sb.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	sb.WriteString(fmt.Sprintf("║ 总交易数:   %4d     | 盈利交易:   %4d     | 亏损交易: %4d ║\n", 
		r.Metrics.TotalTrades, r.Metrics.WinTrades, r.Metrics.LossTrades))
	sb.WriteString(fmt.Sprintf("║ 平均盈利:   %8.2f   | 平均亏损:   %8.2f                   ║\n", 
		r.Metrics.AverageWin, r.Metrics.AverageLoss))
	sb.WriteString(fmt.Sprintf("╚══════════════════════════════════════════════════════════════╝\n"))

	return sb.String()
}

// Print 打印报告
func (r *Report) Print() {
	logger.Info("回测报告",
		zap.String("content", r.ToString()))
}
