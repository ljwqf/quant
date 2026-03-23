package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ljwqf/quant/internal/alertservice"
	"github.com/ljwqf/quant/internal/api"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/dataservice"
	"github.com/ljwqf/quant/internal/exchange/okx"
	"github.com/ljwqf/quant/internal/execution"
	"github.com/ljwqf/quant/internal/llmanalysis"
	"github.com/ljwqf/quant/internal/manualtrading"
	"github.com/ljwqf/quant/internal/monitoring"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

func buildSmartFilterRefreshConfig(cfg *config.Config) *smartFilterRefreshConfig {
	// 从配置文件读取SmartFilter配置
	smartFilterCfg := cfg.Strategy.SmartFilter

	// 设置默认值
	enabled := true
	interval := 5 * time.Minute
	httpTimeout := 10 * time.Second
	source := "auto"
	filePath := "data/smart_filter_data.json"
	httpURL := ""
	cryptoQuantAsset := "btc"

	// 从配置文件覆盖默认值
	if smartFilterCfg.Source != "" {
		source = smartFilterCfg.Source
	}

	// 从配置文件读取CryptoQuant配置
	if smartFilterCfg.CryptoQuant.APIKey != "" {
		// 设置环境变量，供CryptoQuant客户端使用
		os.Setenv("CRYPTOQUANT_API_KEY", smartFilterCfg.CryptoQuant.APIKey)
	}

	if smartFilterCfg.CryptoQuant.Asset != "" {
		cryptoQuantAsset = smartFilterCfg.CryptoQuant.Asset
	}

	return &smartFilterRefreshConfig{
		Enabled:          enabled,
		Source:           source,
		Interval:         interval,
		FilePath:         filePath,
		HTTPURL:          httpURL,
		HTTPTimeout:      httpTimeout,
		CryptoQuantAsset: cryptoQuantAsset,
	}
}

const shutdownTimeout = 30 * time.Second

type managedStrategy struct {
	instance strategy.Strategy
	params   map[string]interface{}
}

type strategyPauser interface {
	Pause()
}

type strategyStopper interface {
	Stop()
}

func invokeStrategyStopHook(instance strategy.Strategy) string {
	if instance == nil {
		return ""
	}

	if pauser, ok := instance.(strategyPauser); ok {
		pauser.Pause()
		return "pause"
	}

	if stopper, ok := instance.(strategyStopper); ok {
		stopper.Stop()
		return "stop"
	}

	return ""
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "配置文件路径")
	flag.Parse()

	fmt.Printf("正在加载配置文件，路径: %s\n", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("配置加载成功，应用名称: %s\n", cfg.Basic.AppName)

	logPath := fmt.Sprintf("logs/%s.log", cfg.Basic.AppName)
	if err := logger.Init(cfg.Basic.LogLevel, logPath); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	defer logger.GetLogger().Sync()

	logger.Info("启动 OKX 量化交易系统",
		zap.String("app_name", cfg.Basic.AppName),
		zap.String("env", cfg.Basic.Env),
		zap.String("log_level", cfg.Basic.LogLevel),
	)

	logger.Info("配置信息",
		zap.String("exchange", "OKX"),
		zap.String("base_url", cfg.Exchange.OKX.BaseURL),
		zap.String("ws_url", cfg.Exchange.OKX.WSURL),
		zap.Float64("max_position_size", cfg.Risk.MaxPositionSize),
		zap.Float64("max_daily_loss", cfg.Risk.MaxDailyLoss),
	)

	logger.Info("系统启动中...")

	exchange := okx.NewClient(&cfg.Exchange.OKX)
	if exchange == nil {
		logger.Error("初始化交易所客户端失败")
		os.Exit(1)
	}

	if err := exchange.Connect(); err != nil {
		logger.Error("连接交易所失败", zap.Error(err))
		os.Exit(1)
	}
	defer exchange.Disconnect()

	var db *storage.Database
	var manualTradeMgr *manualtrading.Manager
	var llmClient *llmanalysis.Client
	var llmAnalyzer *llmanalysis.Analyzer
	var dataService *dataservice.DataService
	var alertService *alertservice.AlertService
	if cfg.Database.Enable {
		db = storage.NewDatabase(&cfg.Database)
		if db == nil {
			logger.Error("初始化数据库失败")
			os.Exit(1)
		}
		defer db.Close()

		if err := db.Migrate(); err != nil {
			logger.Error("数据库迁移失败", zap.Error(err))
			os.Exit(1)
		}

		logger.Info("数据库初始化成功")

		manualTradeMgr = manualtrading.NewManager(&cfg.ManualTrading, db, exchange)
		if manualTradeMgr != nil {
			manualTradeMgr.Start()
			logger.Info("手动交易模块初始化成功")
		}
	}

	if cfg.LLM.Enable {
		llmClient = llmanalysis.NewClient(&cfg.LLM)
		if llmClient != nil {
			logger.Info("大模型客户端初始化成功")
			if db != nil {
				llmAnalyzer = llmanalysis.NewAnalyzer(llmClient, db, &cfg.LLM)
				if llmAnalyzer != nil {
					logger.Info("大模型分析引擎初始化成功")
				}
			}
		}
	}

	if cfg.DataService.Enable && db != nil {
		dataService = dataservice.NewDataService(cfg, db)
		if dataService != nil {
			logger.Info("数据采集服务初始化成功")
			if err := dataService.Start(); err != nil {
				logger.Warn("启动数据采集服务失败", zap.Error(err))
			}
		}
	}

	if cfg.Alert.Enable && db != nil {
		alertService = alertservice.NewAlertService(cfg, db)
		if alertService != nil {
			logger.Info("提醒服务初始化成功")
			if err := alertService.Start(); err != nil {
				logger.Warn("启动提醒服务失败", zap.Error(err))
			}
		}
	}

	riskEngine := risk.NewEngine(&cfg.Risk)

	riskEngine.SetStrategyWeights(map[string]float64{
		"LiquidityHuntEngine":     0.15,
		"BetaArbitrageEngine":     0.1,
		"MMPEngine-Pro":           0.15,
		"DeltaNeutralFunding-Pro": 0.4,
		"NeedleStrategy":          0.2,
	})

	strategyEngine := strategy.NewEngine()
	managedStrategies := map[string]managedStrategy{}

	logger.Info("初始化策略模块...")

	liquidityHuntEngine := strategy.NewLiquidityHuntEngine()
	liquidityHuntParams := map[string]interface{}{
		"fake_break_threshold":   0.3,
		"funding_rate_threshold": 0.0005,
		"time_window":            []string{"20:30", "23:00"},
		"oi_delta_threshold":     50.0,
	}
	if err := strategyEngine.AddStrategy("LiquidityHuntEngine", liquidityHuntEngine, liquidityHuntParams); err != nil {
		logger.Error("添加LiquidityHuntEngine策略失败", zap.Error(err))
	}
	managedStrategies["LiquidityHuntEngine"] = managedStrategy{instance: liquidityHuntEngine, params: liquidityHuntParams}

	betaArbitrageEngine := strategy.NewBetaArbitrageEngine()
	betaArbitrageParams := map[string]interface{}{
		"benchmark":              "BTC-USDT",
		"correlation_period":     24,
		"beta_threshold":         1.5,
		"rsi_period":             14,
		"rsi_threshold":          75,
		"funding_check_interval": 8,
		"max_holding_time":       2,
		"trailing_activation":    1.0,
	}
	if err := strategyEngine.AddStrategy("BetaArbitrageEngine", betaArbitrageEngine, betaArbitrageParams); err != nil {
		logger.Error("添加BetaArbitrageEngine策略失败", zap.Error(err))
	}
	managedStrategies["BetaArbitrageEngine"] = managedStrategy{instance: betaArbitrageEngine, params: betaArbitrageParams}

	mmpEngine := strategy.NewMMPEnginePro()
	mmpParams := map[string]interface{}{
		"mmp_threshold":      0.15,
		"spread_threshold":   0.0003,
		"max_position_value": 10000.0,
		"risk_per_trade":     0.005,
		"hard_stop_loss":     0.005,
		"take_profit_rr":     1.5,
		"signal_ttl":         3 * time.Minute,
		"atr_period":         14,
		"volatility_period":  20,
	}
	if err := strategyEngine.AddStrategy("MMPEngine-Pro", mmpEngine, mmpParams); err != nil {
		logger.Error("添加MMPEngine-Pro策略失败", zap.Error(err))
	}
	managedStrategies["MMPEngine-Pro"] = managedStrategy{instance: mmpEngine, params: mmpParams}

	deltaNeutralEngine := strategy.NewDeltaNeutralFundingPro()
	deltaNeutralParams := map[string]interface{}{
		"spot_symbol":              "BTC-USDT",
		"perp_symbol":              "BTC-USDT-SWAP",
		"fund_usage_percent":       defaultFloat64(cfg.Strategy.DeltaNeutral.FundUsagePercent, strategy.FundUsagePercent),
		"rebalance_threshold":      defaultFloat64(cfg.Strategy.DeltaNeutral.RebalanceThreshold, strategy.RebalanceThreshold),
		"basis_circuit_breaker":    defaultFloat64(cfg.Strategy.DeltaNeutral.BasisCircuitBreaker, strategy.BasisCircuitBreaker),
		"target_hedge_ratio":       defaultFloat64(cfg.Strategy.DeltaNeutral.TargetHedgeRatio, strategy.TargetHedgeRatio),
		"hedge_ratio_tolerance":    defaultFloat64(cfg.Strategy.DeltaNeutral.HedgeRatioTolerance, strategy.HedgeRatioTolerance),
		"daily_loss_limit":         defaultFloat64(cfg.Strategy.DeltaNeutral.DailyLossLimit, strategy.DailyLossLimitFunding),
		"margin_buffer_percent":    defaultFloat64(cfg.Strategy.DeltaNeutral.MarginBufferPercent, strategy.MarginBufferPercent),
		"settlement_window_before": defaultDuration(cfg.Strategy.DeltaNeutral.SettlementWindowBefore, strategy.SettlementWindowBefore),
		"settlement_window_after":  defaultDuration(cfg.Strategy.DeltaNeutral.SettlementWindowAfter, strategy.SettlementWindowAfter),
	}
	if err := strategyEngine.AddStrategy("DeltaNeutralFunding-Pro", deltaNeutralEngine, deltaNeutralParams); err != nil {
		logger.Error("添加DeltaNeutralFunding-Pro策略失败", zap.Error(err))
	}
	managedStrategies["DeltaNeutralFunding-Pro"] = managedStrategy{instance: deltaNeutralEngine, params: deltaNeutralParams}

	needleStrategy := strategy.NewNeedleStrategy()
	needleParams := map[string]interface{}{
		"supertrend_period":     10,
		"supertrend_multiplier": 3.0,
		"macd_fast_period":      12,
		"macd_slow_period":      26,
		"macd_signal_period":    9,
		"needle_distance":       0.003,
		"take_profit_percent":   0.005,
		"stop_loss_percent":     0.008,
		"max_holding_time":      2 * time.Minute,
	}
	if err := strategyEngine.AddStrategy("NeedleStrategy", needleStrategy, needleParams); err != nil {
		logger.Error("添加NeedleStrategy策略失败", zap.Error(err))
	}
	managedStrategies["NeedleStrategy"] = managedStrategy{instance: needleStrategy, params: needleParams}

	// 新增：趋势跟踪策略
	trendFollowingStrategy := strategy.NewTrendFollowingStrategy()
	trendFollowingParams := map[string]interface{}{
		"ema_short_period":     cfg.Strategy.TrendFollowing.EMAShortPeriod,
		"ema_long_period":      cfg.Strategy.TrendFollowing.EMALongPeriod,
		"adx_period":           cfg.Strategy.TrendFollowing.ADXPeriod,
		"adx_threshold":        cfg.Strategy.TrendFollowing.ADXThreshold,
		"trend_strength":       cfg.Strategy.TrendFollowing.TrendStrength,
		"stop_loss_percent":    cfg.Strategy.TrendFollowing.StopLossPercent,
		"trailing_stop_percent": cfg.Strategy.TrendFollowing.TrailingStopPercent,
		"signal_cooldown":      cfg.Strategy.TrendFollowing.SignalCooldown,
	}
	if err := strategyEngine.AddStrategy("TrendFollowingStrategy", trendFollowingStrategy, trendFollowingParams); err != nil {
		logger.Error("添加TrendFollowingStrategy策略失败", zap.Error(err))
	}
	managedStrategies["TrendFollowingStrategy"] = managedStrategy{instance: trendFollowingStrategy, params: trendFollowingParams}

	// 新增：均值回归策略
	meanReversionStrategy := strategy.NewMeanReversionStrategy()
	meanReversionParams := map[string]interface{}{
		"rsi_period":           cfg.Strategy.MeanReversion.RSIPeriod,
		"rsi_overbought":       cfg.Strategy.MeanReversion.RSIOverbought,
		"rsi_oversold":         cfg.Strategy.MeanReversion.RSIOversold,
		"bb_period":            cfg.Strategy.MeanReversion.BBPeriod,
		"bb_std_dev":           cfg.Strategy.MeanReversion.BBStdDev,
		"threshold":            cfg.Strategy.MeanReversion.Threshold,
		"stop_loss_percent":    cfg.Strategy.MeanReversion.StopLossPercent,
		"trailing_stop_percent": cfg.Strategy.MeanReversion.TrailingStopPercent,
		"signal_cooldown":      cfg.Strategy.MeanReversion.SignalCooldown,
	}
	if err := strategyEngine.AddStrategy("MeanReversionStrategy", meanReversionStrategy, meanReversionParams); err != nil {
		logger.Error("添加MeanReversionStrategy策略失败", zap.Error(err))
	}
	managedStrategies["MeanReversionStrategy"] = managedStrategy{instance: meanReversionStrategy, params: meanReversionParams}

	// 新增：波动率突破策略
	volatilityBreakoutStrategy := strategy.NewVolatilityBreakoutStrategy()
	volatilityBreakoutParams := map[string]interface{}{
		"atr_period":           cfg.Strategy.VolatilityBreakout.ATRPeriod,
		"volume_ma_period":     cfg.Strategy.VolatilityBreakout.VolumeMAPeriod,
		"breakout_multiplier":  cfg.Strategy.VolatilityBreakout.BreakoutMultiplier,
		"min_volume_ratio":     cfg.Strategy.VolatilityBreakout.MinVolumeRatio,
		"max_holding_bars":     cfg.Strategy.VolatilityBreakout.MaxHoldingBars,
		"stop_loss_percent":    cfg.Strategy.VolatilityBreakout.StopLossPercent,
		"trailing_stop_percent": cfg.Strategy.VolatilityBreakout.TrailingStopPercent,
		"signal_cooldown":      cfg.Strategy.VolatilityBreakout.SignalCooldown,
	}
	if err := strategyEngine.AddStrategy("VolatilityBreakoutStrategy", volatilityBreakoutStrategy, volatilityBreakoutParams); err != nil {
		logger.Error("添加VolatilityBreakoutStrategy策略失败", zap.Error(err))
	}
	managedStrategies["VolatilityBreakoutStrategy"] = managedStrategy{instance: volatilityBreakoutStrategy, params: volatilityBreakoutParams}

	smartFilter := strategy.NewSmartFilter()
	smartFilterParams := map[string]interface{}{
		"netflow_threshold":   5000.0,
		"sopr_high_threshold": 1.05,
		"sopr_low_threshold":  0.95,
		"mvrv_low_threshold":  1.0,
	}
	if err := smartFilter.Init(smartFilterParams); err != nil {
		logger.Error("初始化SmartFilter失败", zap.Error(err))
	}

	needleStrategy.SetSmartFilter(smartFilter)
	deltaNeutralEngine.SetSmartFilter(smartFilter)
	trendFollowingStrategy.SetSmartFilter(smartFilter)
	meanReversionStrategy.SetSmartFilter(smartFilter)
	volatilityBreakoutStrategy.SetSmartFilter(smartFilter)
	bootstrapNetflow := envFloat64OrDefault("SMART_FILTER_NETFLOW", -6000)
	bootstrapSOPR := envFloat64OrDefault("SMART_FILTER_SOPR", 0.94)
	bootstrapMVRV := envFloat64OrDefault("SMART_FILTER_MVRV", 0.95)
	needleStrategy.UpdateOnChainData(bootstrapNetflow, bootstrapSOPR, bootstrapMVRV)
	deltaNeutralEngine.UpdateOnChainData(bootstrapNetflow, bootstrapSOPR, bootstrapMVRV)
	logger.Info("SmartFilter已注入初始链上数据",
		zap.Float64("netflow", bootstrapNetflow),
		zap.Float64("sopr", bootstrapSOPR),
		zap.Float64("mvrv", bootstrapMVRV),
	)

	// 构建SmartFilter刷新配置
	smartFilterRefreshCfg := buildSmartFilterRefreshConfig(cfg)

	// 启动SmartFilter自动刷新
	stopSmartFilterRefresh := startSmartFilterAutoRefresh(smartFilter, smartFilterRefreshCfg, needleStrategy, deltaNeutralEngine, trendFollowingStrategy, meanReversionStrategy, volatilityBreakoutStrategy)
	defer stopSmartFilterRefresh()

	allocatorParams := map[string]interface{}{
		"min_weight":              defaultFloat64(cfg.Execution.Allocator.MinWeight, strategy.MinWeight),
		"max_weight":              defaultFloat64(cfg.Execution.Allocator.MaxWeight, strategy.MaxWeight),
		"weight_change_threshold": defaultFloat64(cfg.Execution.Allocator.WeightChangeThreshold, strategy.WeightChangeThreshold),
		"cooldown_duration":       defaultDuration(cfg.Execution.Allocator.CooldownDuration, strategy.CooldownDuration),
		"rebalance_interval":      defaultDuration(cfg.Execution.Allocator.RebalanceInterval, strategy.RebalanceInterval),
		"portfolio_loss_limit":    defaultFloat64(cfg.Execution.Allocator.PortfolioLossLimit, strategy.PortfolioLossLimit),
	}

	logger.Info("策略模块初始化完成",
		zap.Strings("strategies", []string{
			"LiquidityHuntEngine",
			"BetaArbitrageEngine",
			"MMPEngine-Pro",
			"DeltaNeutralFunding-Pro",
			"NeedleStrategy",
			"TrendFollowingStrategy",
			"MeanReversionStrategy",
			"VolatilityBreakoutStrategy",
		}),
		zap.Strings("auxiliary_modules", []string{
			"SmartFilter",
			"OnlineBayesianAllocator",
		}),
	)

	takeProfitConfig := &execution.TakeProfitConfig{
		Enabled:            true,
		Type:               execution.TakeProfitTypeTrailing,
		TrailingActivation: 1.0,
		TrailingDistance:   0.5,
		TrailingStep:       0.3,
		ATRPeriod:          14,
		ATRMultiplier:      2.0,
		MaxHoldingTime:     2 * time.Hour,
		PullbackPercent:    0.3,
		TieredLevels: []execution.TieredLevel{
			{ProfitPercent: 1.5, ClosePercent: 30},
			{ProfitPercent: 3.0, ClosePercent: 40},
			{ProfitPercent: 5.0, ClosePercent: 30},
		},
	}

	engineConfig := &execution.EngineConfig{
		TakeProfitConfig:         takeProfitConfig,
		EnableStrategyTakeProfit: cfg.Execution.EnableStrategyTakeProfit,
		SmartRouteConfig: execution.SmartRouteConfig{
			OrderBookDepth:       cfg.Execution.SmartRouting.OrderBookDepth,
			MaxEstimatedSlippage: cfg.Execution.SmartRouting.MaxEstimatedSlippage,
		},
		RebalanceConfig: execution.RebalanceConfig{
			Enabled:              cfg.Execution.Rebalance.Enabled,
			ReduceOnly:           cfg.Execution.Rebalance.ReduceOnly,
			DriftThreshold:       cfg.Execution.Rebalance.DriftThreshold,
			UseMarketOrders:      cfg.Execution.Rebalance.UseMarketOrders,
			MaxPositionsPerCycle: cfg.Execution.Rebalance.MaxPositionsPerCycle,
			CircuitAutoReset:     cfg.Execution.Rebalance.CircuitAutoReset,
			CircuitCooldown:      cfg.Execution.Rebalance.CircuitCooldown,
		},
	}

	executionEngine := execution.NewEngineWithConfig(exchange, riskEngine, strategyEngine, engineConfig)
	metrics := monitoring.NewMetrics(&monitoring.MetricsConfig{
		Enable:         cfg.Monitoring.Metrics.Enable,
		PushGatewayURL: cfg.Monitoring.Metrics.PushGatewayURL,
	})
	if err := metrics.Start(); err != nil {
		logger.Error("启动指标收集失败", zap.Error(err))
	}
	defer metrics.Stop()

	alertManager := monitoring.NewAlertManager(&monitoring.AlertConfig{
		Enable:      cfg.Monitoring.Alert.Enable,
		Channels:    append([]string(nil), cfg.Monitoring.Alert.Channels...),
		WebhookURL:  cfg.Monitoring.Alert.WebhookURL,
		DedupWindow: cfg.Monitoring.Alert.DedupWindow,
		MinInterval: cfg.Monitoring.Alert.MinInterval,
	})
	if err := alertManager.Start(); err != nil {
		logger.Error("启动告警管理器失败", zap.Error(err))
	}
	defer alertManager.Stop()

	var apiServer *api.Server

	executionEngine.SetAlertHandler(func(level execution.AlertLevel, title, message string, labels map[string]string, details map[string]interface{}) {
		normalizedLabels, normalizedDetails := normalizeRebalanceRoutingFields("", labels, details)
		if err := alertManager.AlertWithContext(mapExecutionAlertLevel(level), title, message, normalizedLabels, normalizedDetails); err != nil {
			logger.Warn("发送执行层告警失败", zap.Error(err), zap.String("title", title))
		}
	})
	executionEngine.SetRebalanceEventHandler(func(event execution.RebalanceEvent) {
		if apiServer == nil {
			return
		}
		apiServer.UpdateRebalanceCircuit(rebalanceCircuitInfoFromState(event.Circuit))
		apiServer.BroadcastRebalanceEvent(rebalanceEventInfoFromExecution(event))
	})
	allocator := executionEngine.GetBayesianAllocator()
	if err := allocator.Init(allocatorParams); err != nil {
		logger.Error("初始化OnlineBayesianAllocator失败", zap.Error(err))
	}
	allocator.RegisterStrategy("MMPEngine-Pro", 0.3)
	allocator.RegisterStrategy("DeltaNeutralFunding-Pro", 0.5)
	allocator.RegisterStrategy("NeedleStrategy", 0.2)

	var snapshotTicker *time.Ticker
	var snapshotStop chan struct{}
	if cfg.Execution.Persistence.Enabled {
		stateStore, err := execution.NewFileStateStore(cfg.Execution.Persistence.Directory)
		if err != nil {
			logger.Error("初始化执行态存储失败", zap.Error(err))
			os.Exit(1)
		}

		executionEngine.SetStateStore(stateStore)

		loaded, err := executionEngine.LoadStateSnapshot()
		if err != nil {
			logger.Error("加载执行态快照失败", zap.Error(err))
		} else if loaded {
			logger.Info("已恢复执行态快照", zap.String("directory", cfg.Execution.Persistence.Directory))
		}

		if err := executionEngine.ReconcileWithExchange(); err != nil {
			logger.Error("启动对账失败", zap.Error(err))
		} else {
			logger.Info("启动对账完成")
		}

		snapshotTicker = time.NewTicker(cfg.Execution.Persistence.SnapshotInterval)
		snapshotStop = make(chan struct{})
		go func() {
			for {
				select {
				case <-snapshotTicker.C:
					if err := executionEngine.SaveStateSnapshot(); err != nil {
						logger.Error("保存执行态快照失败", zap.Error(err))
					}
				case <-snapshotStop:
					return
				}
			}
		}()
	}

	executionEngine.StartTakeProfitMonitor()
	executionEngine.StartOrderMonitor()

	executeSignal := func(signal *types.Signal) {
		logger.Info("策略信号触发",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol),
			zap.String("type", string(signal.Type)),
			zap.Float64("price", signal.Price),
		)

		account, err := exchange.GetAccount()
		if err != nil {
			logger.Error("获取账户信息失败", zap.Error(err))
			return
		}

		result, err := executionEngine.Execute(signal, account.TotalAvailable)
		if err != nil {
			logger.Error("执行信号失败",
				zap.String("strategy", signal.Strategy),
				zap.Error(err))
			return
		}

		if result != nil {
			logger.Info("订单执行成功",
				zap.String("strategy", signal.Strategy),
				zap.String("order_id", result.OrderID),
				zap.String("status", string(result.Status)),
			)
		}
	}

	needleStrategy.SetSignalCallback(executeSignal)

	logger.Info("动态止盈配置",
		zap.String("type", string(takeProfitConfig.Type)),
		zap.Float64("trailing_activation", takeProfitConfig.TrailingActivation),
		zap.Float64("trailing_distance", takeProfitConfig.TrailingDistance),
		zap.Duration("max_holding_time", takeProfitConfig.MaxHoldingTime),
	)

	realTimePnL := monitoring.NewRealTimePnL(exchange, riskEngine, executionEngine, strategyEngine, metrics, alertManager)

	if err := realTimePnL.Start(); err != nil {
		logger.Error("启动实时P&L监控失败", zap.Error(err))
	}
	defer realTimePnL.Stop()

	symbol := "BTC-USDT"
	barInterval := "1m"

	logger.Info("订阅行情数据...",
		zap.String("symbol", symbol),
		zap.String("bar_interval", barInterval),
	)

	if err := subscribeStrategyMarketData(exchange, symbol, barInterval, strategyEngine, executeSignal); err != nil {
		logger.Error("订阅行情数据失败", zap.Error(err))
	}

	logger.Info("系统启动完成，等待交易信号...")

	logger.Info("策略部署优先级说明",
		zap.String("第一优先级", "DeltaNeutralFunding-Pro（风险最低，建立基础收益）"),
		zap.String("第二优先级", "NeedleStrategy（插针策略，分钟级别MACD背离）"),
		zap.String("第三优先级", "TrendFollowingStrategy（趋势跟踪，EMA+ADX）"),
		zap.String("第四优先级", "MeanReversionStrategy（均值回归，RSI+布林带）"),
		zap.String("第五优先级", "VolatilityBreakoutStrategy（波动率突破，ATR+成交量）"),
		zap.String("第六优先级", "MMPEngine-Pro（限制总仓位<20%）"),
		zap.String("第七优先级", "SmartFilter（作为策略准入过滤器）"),
		zap.String("第八优先级", "OnlineBayesianAllocator（管理策略资金权重）"),
	)

	logger.Info("风控权重配置",
		zap.Any("weights", riskEngine.GetStrategyWeights()),
	)

	logger.Info("策略配置说明",
		zap.String("NeedleStrategy", "插针策略（超级趋势+MACD背离）"),
		zap.String("TrendFollowingStrategy", "趋势跟踪（EMA12/26 + ADX14）"),
		zap.String("MeanReversionStrategy", "均值回归（RSI + 布林带）"),
		zap.String("VolatilityBreakoutStrategy", "波动率突破（ATR + 成交量）"),
	)

	buildStrategyStatus := func(name string, running bool) *api.StrategyStatus {
		metrics := strategyEngine.GetStrategyMetrics(name)
		status := &api.StrategyStatus{
			Name:       name,
			Enabled:    true,
			Running:    running,
			LastUpdate: time.Now(),
		}
		if metrics != nil {
			if pnl, ok := metrics["pnl"].(float64); ok {
				status.PnL = pnl
			}
			if winRate, ok := metrics["win_rate"].(float64); ok {
				status.WinRate = winRate
			}
			if trades, ok := metrics["total_trades"].(int); ok {
				status.Trades = trades
			}
			if lastSignal, ok := metrics["last_signal"].(string); ok {
				status.LastSignal = lastSignal
			}
		}
		return status
	}

	startStrategy := func(name string) (*api.StrategyStatus, error) {
		managed, ok := managedStrategies[name]
		if !ok {
			return nil, fmt.Errorf("策略不存在: %s", name)
		}
		if strategyEngine.HasStrategy(name) {
			return buildStrategyStatus(name, true), nil
		}
		if err := strategyEngine.AddStrategy(name, managed.instance, managed.params); err != nil {
			return nil, err
		}
		return buildStrategyStatus(name, true), nil
	}

	stopStrategy := func(name string) (*api.StrategyStatus, error) {
		managed, ok := managedStrategies[name]
		if !ok {
			return nil, fmt.Errorf("策略不存在: %s", name)
		}
		if strategyEngine.HasStrategy(name) {
			if hook := invokeStrategyStopHook(managed.instance); hook != "" {
				logger.Info("已调用策略停机钩子",
					zap.String("strategy", name),
					zap.String("hook", hook),
				)
			}
			strategyEngine.RemoveStrategy(name)
		}
		return buildStrategyStatus(name, false), nil
	}

	createOrder := func(order *types.Order) (*types.OrderResult, error) {
		return executionEngine.PlaceOrder(order)
	}

	closePosition := func(symbol string) (*types.OrderResult, error) {
		return executionEngine.ClosePosition(strings.TrimSpace(symbol), 0)
	}

	getRebalanceCircuit := func() (*api.RebalanceCircuitInfo, error) {
		state := executionEngine.GetRebalanceCircuitState()
		return rebalanceCircuitInfoFromState(state), nil
	}

	resetRebalanceCircuit := func(reason string) (*api.RebalanceCircuitInfo, error) {
		resetReason := strings.TrimSpace(reason)
		if resetReason == "" {
			resetReason = "api_manual_reset"
		}
		preResetState := executionEngine.GetRebalanceCircuitState()
		if !executionEngine.ResetRebalanceCircuit(resetReason) {
			return getRebalanceCircuit()
		}
		labels, details := normalizeRebalanceRoutingFields("reset", map[string]string{
			"component": "execution",
			"event":     "rebalance_circuit_reset_manual",
			"strategy":  preResetState.Strategy,
			"step":      preResetState.Step,
			"reason":    resetReason,
		}, map[string]interface{}{
			"strategy":   preResetState.Strategy,
			"step":       preResetState.Step,
			"reason":     resetReason,
			"reset_mode": "manual",
		})
		if err := alertManager.AlertWithContext(monitoring.AlertTypeInfo, "再平衡熔断已手动重置", fmt.Sprintf("原因: %s", resetReason), labels, details); err != nil {
			logger.Warn("发送重置熔断告警失败", zap.Error(err))
		}
		return getRebalanceCircuit()
	}

	getTicker := func(symbol string) (*types.Tick, error) {
		return exchange.GetTicker(symbol)
	}

	getBars := func(symbol string, interval string, limit int) ([]*types.Bar, error) {
		return exchange.GetBars(symbol, interval, limit)
	}

	getOrderBook := func(symbol string, depth int) (*types.OrderBook, error) {
		return exchange.GetOrderBook(symbol, depth)
	}

	// 启动API服务器
	apiServer = api.NewServer(cfg.Server.Host, cfg.Server.Port, cfg, configPath, &api.ActionHandlers{
		StartStrategy:         startStrategy,
		StopStrategy:          stopStrategy,
		CreateOrder:           createOrder,
		ClosePosition:         closePosition,
		GetRebalanceCircuit:   getRebalanceCircuit,
		ResetRebalanceCircuit: resetRebalanceCircuit,
		GetTicker:             getTicker,
		GetBars:               getBars,
		GetOrderBook:          getOrderBook,
	})
	for name := range managedStrategies {
		apiServer.UpdateStrategyStatus(name, buildStrategyStatus(name, strategyEngine.HasStrategy(name)))
	}
	if manualTradeMgr != nil {
		apiServer.SetManualTradeManager(manualTradeMgr)
	}
	if llmAnalyzer != nil {
		apiServer.SetAnalyzer(llmAnalyzer)
	}
	if dataService != nil {
		apiServer.SetDataService(dataService)
	}
	if alertService != nil {
		apiServer.SetAlertService(alertService)
	}
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("API服务器启动失败", zap.Error(err))
		}
	}()
	defer apiServer.Stop()

	dashboardHost := cfg.Server.Host
	if dashboardHost == "0.0.0.0" {
		dashboardHost = "127.0.0.1"
	}
	logger.Info("Dashboard已启动", zap.String("url", fmt.Sprintf("http://%s:%d", dashboardHost, cfg.Server.Port)))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭系统...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var shutdownWg sync.WaitGroup

	if snapshotTicker != nil {
		snapshotTicker.Stop()
	}
	if snapshotStop != nil {
		close(snapshotStop)
	}

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		if cfg.Execution.Persistence.Enabled {
			if err := executionEngine.SaveStateSnapshot(); err != nil {
				logger.Error("关闭前保存执行态快照失败", zap.Error(err))
			} else {
				logger.Info("执行态快照已保存")
			}
		}
	}()

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		mmpEngine.Stop()
		deltaNeutralEngine.Stop()
		needleStrategy.Stop()
		logger.Info("策略已停止")
	}()

	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		if dataService != nil {
			dataService.Stop()
			logger.Info("数据采集服务已停止")
		}
		if alertService != nil {
			alertService.Stop()
			logger.Info("提醒服务已停止")
		}
		exchange.Disconnect()
		logger.Info("交易所连接已断开")
	}()

	done := make(chan struct{})
	go func() {
		shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		logger.Warn("关闭超时，强制退出")
	case <-done:
		logger.Info("所有组件已优雅关闭")
	}

	logger.Info("系统已关闭")
}

func defaultFloat64(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func mapExecutionAlertLevel(level execution.AlertLevel) monitoring.AlertType {
	switch level {
	case execution.AlertLevelInfo:
		return monitoring.AlertTypeInfo
	case execution.AlertLevelWarning:
		return monitoring.AlertTypeWarning
	case execution.AlertLevelError:
		return monitoring.AlertTypeError
	case execution.AlertLevelCritical:
		return monitoring.AlertTypeCritical
	default:
		return monitoring.AlertTypeInfo
	}
}

func rebalanceCircuitInfoFromState(state execution.RebalanceCircuitState) *api.RebalanceCircuitInfo {
	return &api.RebalanceCircuitInfo{
		Open:            state.Open,
		Strategy:        state.Strategy,
		Step:            state.Step,
		Reason:          state.Reason,
		OpenedAt:        state.OpenedAt,
		CooldownUntil:   state.CooldownUntil,
		LastResetAt:     state.LastResetAt,
		LastResetReason: state.LastResetReason,
		AutoReset:       state.AutoReset,
		Cooldown:        state.Cooldown.String(),
	}
}

func rebalanceEventInfoFromExecution(event execution.RebalanceEvent) *api.RebalanceEventInfo {
	if event.Type == "" && event.Message == "" && event.Timestamp.IsZero() {
		return nil
	}
	labels, details := normalizeRebalanceRoutingFields(string(event.Type), event.Labels, event.Details)
	return &api.RebalanceEventInfo{
		Type:      string(event.Type),
		Strategy:  event.Strategy,
		Step:      event.Step,
		Reason:    event.Reason,
		Message:   event.Message,
		Timestamp: event.Timestamp,
		Labels:    labels,
		Details:   details,
		Circuit:   rebalanceCircuitInfoFromState(event.Circuit),
	}
}

func normalizeRebalanceRoutingFields(eventType string, labels map[string]string, details map[string]interface{}) (map[string]string, map[string]interface{}) {
	normalizedLabels := cloneStringMapMain(labels)
	normalizedDetails := cloneInterfaceMapMain(details)
	if normalizedLabels == nil {
		normalizedLabels = map[string]string{}
	}
	if normalizedDetails == nil {
		normalizedDetails = map[string]interface{}{}
	}

	inferredType := strings.TrimSpace(eventType)
	if inferredType == "" {
		inferredType = inferRebalanceEventType(normalizedLabels)
	}
	if inferredType == "" {
		return normalizedLabels, normalizedDetails
	}

	strategyName := firstNonEmptyString(normalizedLabels["strategy"], stringDetail(normalizedDetails["strategy"]))
	step := firstNonEmptyString(normalizedLabels["step"], stringDetail(normalizedDetails["step"]))
	reason := firstNonEmptyString(normalizedLabels["reason"], stringDetail(normalizedDetails["reason"]))
	resetMode := firstNonEmptyString(normalizedLabels["reset_mode"], stringDetail(normalizedDetails["reset_mode"]))
	if inferredType == "reset" && resetMode == "" {
		resetMode = inferResetMode(reason)
	}

	normalizedLabels["component"] = firstNonEmptyString(normalizedLabels["component"], "execution")
	normalizedLabels["domain"] = "rebalance"
	normalizedLabels["event_type"] = inferredType
	normalizedLabels["route"] = buildRebalanceRoute(inferredType, resetMode)
	if strategyName != "" {
		normalizedLabels["strategy"] = strategyName
		normalizedDetails["strategy"] = strategyName
	}
	if step != "" {
		normalizedLabels["step"] = step
		normalizedDetails["step"] = step
	}
	if reason != "" {
		normalizedLabels["reason"] = reason
		normalizedDetails["reason"] = reason
	}
	if resetMode != "" {
		normalizedLabels["reset_mode"] = resetMode
		normalizedDetails["reset_mode"] = resetMode
	}
	normalizedDetails["event_type"] = inferredType
	normalizedDetails["route"] = normalizedLabels["route"]
	return normalizedLabels, normalizedDetails
}

func inferRebalanceEventType(labels map[string]string) string {
	switch labels["event"] {
	case "rebalance_circuit_open":
		return "open"
	case "rebalance_recover_started":
		return "recover_started"
	case "rebalance_recover_succeeded":
		return "recover_succeeded"
	case "rebalance_recover_failed":
		return "recover_failed"
	case "rebalance_circuit_reset_manual", "rebalance_circuit_reset":
		return "reset"
	default:
		return ""
	}
}

func buildRebalanceRoute(eventType, resetMode string) string {
	if eventType == "reset" && resetMode != "" {
		return "rebalance/reset/" + resetMode
	}
	if strings.HasPrefix(eventType, "recover_") {
		return "rebalance/recover/" + strings.TrimPrefix(eventType, "recover_")
	}
	return "rebalance/" + eventType
}

func inferResetMode(reason string) string {
	if strings.TrimSpace(reason) == "cooldown_elapsed" {
		return "automatic"
	}
	return "manual"
}

func stringDetail(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneStringMapMain(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneInterfaceMapMain(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func envFloat64OrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Warn("解析环境变量失败，使用默认值",
			zap.String("key", key),
			zap.String("value", value),
			zap.Float64("fallback", fallback),
			zap.Error(err),
		)
		return fallback
	}

	return parsed
}
