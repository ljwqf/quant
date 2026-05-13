package execution

import (
	"fmt"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// OrderReconciler 订单对账服务
// 定期拉取交易所订单/条件单，与本地状态比对并自动修复不一致
type OrderReconciler struct {
	exch         exchange.Exchange
	riskEngine   *risk.Engine
	execEngine   *Engine
	symbols      []string
	interval     time.Duration
	stopCh       chan struct{}
	once         sync.Once

	// 指标
	totalReconciles   int64
	discrepanciesFound int64
	discrepanciesFixed int64
	metricsMu         sync.Mutex
}

// OrderReconcilerConfig 对账服务配置
type OrderReconcilerConfig struct {
	Enabled  bool
	Symbols  []string
	Interval time.Duration
}

// NewOrderReconciler 创建订单对账服务
func NewOrderReconciler(exch exchange.Exchange, riskEngine *risk.Engine, execEngine *Engine, cfg *OrderReconcilerConfig) *OrderReconciler {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &OrderReconciler{
		exch:       exch,
		riskEngine: riskEngine,
		execEngine: execEngine,
		symbols:    cfg.Symbols,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动对账服务
func (r *OrderReconciler) Start() {
	go r.run()
	logger.Info("订单对账服务已启动",
		zap.Duration("interval", r.interval),
		zap.Strings("symbols", r.symbols))
}

// Stop 停止对账服务
func (r *OrderReconciler) Stop() {
	r.once.Do(func() {
		close(r.stopCh)
		logger.Info("订单对账服务已停止")
	})
}

func (r *OrderReconciler) run() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	r.reconcile()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reconcile()
		}
	}
}

func (r *OrderReconciler) reconcile() {
	r.metricsMu.Lock()
	r.totalReconciles++
	r.metricsMu.Unlock()

	for _, symbol := range r.symbols {
		r.reconcileSymbol(symbol)
	}
}

func (r *OrderReconciler) reconcileSymbol(symbol string) {
	// 1. 对账普通订单
	if err := r.reconcileRegularOrders(symbol); err != nil {
		logger.Warn("普通订单对账失败",
			zap.String("symbol", symbol),
			zap.Error(err))
	}

	// 2. 对账算法单（条件单/止盈止损）
	if err := r.reconcileAlgoOrders(symbol); err != nil {
		logger.Warn("算法单对账失败",
			zap.String("symbol", symbol),
			zap.Error(err))
	}
}

func (r *OrderReconciler) reconcileRegularOrders(symbol string) error {
	// 从交易所拉取最近 50 笔订单
	exchangeOrders, err := r.exch.GetOrders(symbol, 50)
	if err != nil {
		return fmt.Errorf("拉取交易所订单失败: %w", err)
	}

	localOrders := r.execEngine.GetOrders()
	localOrderIDs := make(map[string]bool)
	for id := range localOrders {
		localOrderIDs[id] = true
	}

	for _, exOrder := range exchangeOrders {
		// 跳过已过久的订单
		if time.Since(exOrder.Timestamp) > 24*time.Hour {
			continue
		}

		if _, ok := localOrders[exOrder.ID]; !ok {
			// 交易所有但本地没有的订单 → 孤儿订单
			r.metricsMu.Lock()
			r.discrepanciesFound++
			r.metricsMu.Unlock()

			logger.Warn("发现孤儿订单",
				zap.String("order_id", exOrder.ID),
				zap.String("symbol", exOrder.Symbol),
				zap.String("status", string(exOrder.Status)),
				zap.Float64("filled_qty", exOrder.FilledQty))

			// 如果订单仍在活跃状态，尝试同步到本地
			if exOrder.Status == types.OrderStatusPending || exOrder.Status == types.OrderStatusPartially {
				if err := r.syncOrphanedOrder(exOrder); err != nil {
					logger.Error("同步孤儿订单失败",
						zap.String("order_id", exOrder.ID),
						zap.Error(err))
					continue
				}
				r.metricsMu.Lock()
				r.discrepanciesFixed++
				r.metricsMu.Unlock()
				logger.Info("孤儿订单已同步",
					zap.String("order_id", exOrder.ID))
			}
		}
	}

	// 检查本地有但交易所没有的订单（可能已被清理）
	for id, localOrder := range localOrders {
		if time.Since(localOrder.Timestamp) > 24*time.Hour {
			continue
		}
		found := false
		for _, exOrder := range exchangeOrders {
			if exOrder.ID == id {
				found = true
				break
			}
		}
		if !found && (localOrder.Status == types.OrderStatusPending || localOrder.Status == types.OrderStatusPartially) {
			// 本地活跃但交易所不存在 → 可能已过期或被取消
			r.metricsMu.Lock()
			r.discrepanciesFound++
			r.metricsMu.Unlock()

			logger.Warn("本地活跃订单在交易所不存在",
				zap.String("order_id", id),
				zap.String("symbol", localOrder.Symbol),
				zap.String("local_status", string(localOrder.Status)))
		}
	}

	return nil
}

func (r *OrderReconciler) syncOrphanedOrder(exOrder *types.Order) error {
	// 将孤儿订单注入执行引擎的订单追踪
	r.execEngine.TrackExternalOrder(exOrder)
	return nil
}

func (r *OrderReconciler) reconcileAlgoOrders(symbol string) error {
	// 拉取交易所条件单
	algoOrders, err := r.exch.GetAlgoOrders(symbol, "conditional")
	if err != nil {
		return fmt.Errorf("拉取交易所条件单失败: %w", err)
	}

	localAlgoOrders := r.execEngine.GetAlgoOrders()
	localAlgoIDs := make(map[string]bool)
	for id := range localAlgoOrders {
		localAlgoIDs[id] = true
	}

	for _, exAlgo := range algoOrders {
		if _, ok := localAlgoOrders[exAlgo.AlgoID]; !ok {
			r.metricsMu.Lock()
			r.discrepanciesFound++
			r.metricsMu.Unlock()

			logger.Warn("发现孤儿条件单",
				zap.String("algo_id", exAlgo.AlgoID),
				zap.String("symbol", exAlgo.Symbol),
				zap.String("state", exAlgo.State))

			// 同步活跃条件单
			if exAlgo.State == "live" || exAlgo.State == "effective" {
				r.execEngine.TrackExternalAlgoOrder(exAlgo)
				r.metricsMu.Lock()
				r.discrepanciesFixed++
				r.metricsMu.Unlock()

				logger.Info("孤儿条件单已同步",
					zap.String("algo_id", exAlgo.AlgoID))
			}
		}
	}

	return nil
}

// GetMetrics 获取对账指标
func (r *OrderReconciler) GetMetrics() map[string]interface{} {
	r.metricsMu.Lock()
	defer r.metricsMu.Unlock()
	return map[string]interface{}{
		"total_reconciles":    r.totalReconciles,
		"discrepancies_found": r.discrepanciesFound,
		"discrepancies_fixed": r.discrepanciesFixed,
	}
}
