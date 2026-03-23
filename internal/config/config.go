package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构体
type Config struct {
	Basic         BasicConfig         `mapstructure:"basic"`
	Exchange      ExchangeConfig      `mapstructure:"exchange"`
	Risk          RiskConfig          `mapstructure:"risk"`
	Execution     ExecutionConfig     `mapstructure:"execution"`
	Backtest      BacktestConfig      `mapstructure:"backtest"`
	Monitoring    MonitoringConfig    `mapstructure:"monitoring"`
	Strategy      StrategyConfig      `mapstructure:"strategy"`
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	ManualTrading ManualTradingConfig `mapstructure:"manual_trading"`
	LLM           LLMConfig           `mapstructure:"llm"`
	DataCollector DataCollectorConfig `mapstructure:"data_collector"`
	Alert         AlertServiceConfig  `mapstructure:"alert"`
	DataService   DataServiceConfig   `mapstructure:"data_service"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Enable bool   `mapstructure:"enable"`
	Type   string `mapstructure:"type"`
	Path   string `mapstructure:"path"`
}

// ManualTradingConfig 手动交易配置
type ManualTradingConfig struct {
	Enable            bool `mapstructure:"enable"`
	RiskCheck         bool `mapstructure:"risk_check"`
	OrderConfirmation bool `mapstructure:"order_confirmation"`
}

// LLMConfig 大模型配置
type LLMConfig struct {
	Enable    bool              `mapstructure:"enable"`
	Provider  string            `mapstructure:"provider"`
	Providers LLMProvidersConfig `mapstructure:"providers"`
	Timeout   time.Duration     `mapstructure:"timeout"`
}

// LLMProvidersConfig 各LLM提供商配置
type LLMProvidersConfig struct {
	OpenAI LLMProviderConfig `mapstructure:"openai"`
	Claude LLMProviderConfig `mapstructure:"claude"`
	Qwen   LLMProviderConfig `mapstructure:"qwen"`
}

// LLMProviderConfig 单个LLM提供商配置
type LLMProviderConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Model       string        `mapstructure:"model"`
	Temperature float64       `mapstructure:"temperature"`
	MaxTokens   int           `mapstructure:"max_tokens"`
}

// DataCollectorConfig 数据采集配置
type DataCollectorConfig struct {
	News     NewsCollectorConfig     `mapstructure:"news"`
	Economic EconomicCollectorConfig `mapstructure:"economic"`
}

// NewsCollectorConfig 新闻采集配置
type NewsCollectorConfig struct {
	Enable          bool          `mapstructure:"enable"`
	Sources         []string      `mapstructure:"sources"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// EconomicCollectorConfig 经济数据采集配置
type EconomicCollectorConfig struct {
	Enable          bool          `mapstructure:"enable"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// AlertServiceConfig 提醒服务配置
type AlertServiceConfig struct {
	Enable               bool          `mapstructure:"enable"`
	Channels             []string      `mapstructure:"channels"`
	PriceChangeThreshold float64       `mapstructure:"price_change_threshold"`
	CheckInterval        time.Duration `mapstructure:"check_interval"`
}

// DataServiceConfig 数据采集服务配置
type DataServiceConfig struct {
	Enable             bool          `mapstructure:"enable"`
	NewsEnable         bool          `mapstructure:"news_enable"`
	EconomicEnable     bool          `mapstructure:"economic_enable"`
	CryptoQuantEnable  bool          `mapstructure:"cryptoquant_enable"`
	CryptoQuantAPIKey  string        `mapstructure:"cryptoquant_api_key"`
	Interval           time.Duration `mapstructure:"interval"`
}

// BasicConfig 基本配置
type BasicConfig struct {
	AppName  string `mapstructure:"app_name"`
	Env      string `mapstructure:"env"`
	LogLevel string `mapstructure:"log_level"`
	LogFile  string `mapstructure:"log_file"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	OKX OKXConfig `mapstructure:"okx"`
}

// OKXConfig OKX交易所配置
type OKXConfig struct {
	APIKey     string        `mapstructure:"api_key"`
	SecretKey  string        `mapstructure:"secret_key"`
	Passphrase string        `mapstructure:"passphrase"`
	BaseURL    string        `mapstructure:"base_url"`
	WSURL      string        `mapstructure:"ws_url"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RetryCount int           `mapstructure:"retry_count"`
	Simulated  bool          `mapstructure:"simulated"` // 是否使用模拟盘
}

// RiskConfig 风险管理配置
type RiskConfig struct {
	Enable            bool    `mapstructure:"enable"`
	MaxPositionSize   float64 `mapstructure:"max_position_size"`
	MaxDailyLoss      float64 `mapstructure:"max_daily_loss"`
	MaxDrawdown       float64 `mapstructure:"max_drawdown"`
	StopLossPercent   float64 `mapstructure:"stop_loss_percent"`
	TakeProfitPercent float64 `mapstructure:"take_profit_percent"`
	MaxTradesPerDay   int     `mapstructure:"max_trades_per_day"`
}

// ExecutionConfig 执行引擎配置
type ExecutionConfig struct {
	EnableStrategyTakeProfit bool                       `mapstructure:"enable_strategy_take_profit"`
	Persistence              ExecutionPersistenceConfig `mapstructure:"persistence"`
	SmartRouting             SmartRoutingConfig         `mapstructure:"smart_routing"`
	Rebalance                RebalanceActionConfig      `mapstructure:"rebalance"`
	Allocator                BayesianAllocatorConfig    `mapstructure:"allocator"`
}

// ExecutionPersistenceConfig 执行态持久化配置
type ExecutionPersistenceConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	Directory        string        `mapstructure:"directory"`
	SnapshotInterval time.Duration `mapstructure:"snapshot_interval"`
}

// SmartRoutingConfig 智能路由配置
type SmartRoutingConfig struct {
	OrderBookDepth       int     `mapstructure:"order_book_depth"`
	MaxEstimatedSlippage float64 `mapstructure:"max_estimated_slippage"`
}

// RebalanceActionConfig 再平衡执行配置
type RebalanceActionConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	ReduceOnly           bool          `mapstructure:"reduce_only"`
	DriftThreshold       float64       `mapstructure:"drift_threshold"`
	UseMarketOrders      bool          `mapstructure:"use_market_orders"`
	MaxPositionsPerCycle int           `mapstructure:"max_positions_per_cycle"`
	CircuitAutoReset     bool          `mapstructure:"circuit_auto_reset"`
	CircuitCooldown      time.Duration `mapstructure:"circuit_cooldown"`
}

// BayesianAllocatorConfig 贝叶斯分配器配置
type BayesianAllocatorConfig struct {
	MinWeight             float64       `mapstructure:"min_weight"`
	MaxWeight             float64       `mapstructure:"max_weight"`
	WeightChangeThreshold float64       `mapstructure:"weight_change_threshold"`
	CooldownDuration      time.Duration `mapstructure:"cooldown_duration"`
	RebalanceInterval     time.Duration `mapstructure:"rebalance_interval"`
	PortfolioLossLimit    float64       `mapstructure:"portfolio_loss_limit"`
}

// BacktestConfig 回测配置
type BacktestConfig struct {
	Enable         bool    `mapstructure:"enable"`
	InitialBalance float64 `mapstructure:"initial_balance"`
	DataDir        string  `mapstructure:"data_dir"`
	ResultDir      string  `mapstructure:"result_dir"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enable         bool           `mapstructure:"enable"`
	CheckInterval  time.Duration  `mapstructure:"check_interval"`
	AlertThreshold AlertThreshold `mapstructure:"alert_threshold"`
	Metrics        MetricsConfig  `mapstructure:"metrics"`
	Alert          AlertConfig    `mapstructure:"alert"`
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	MaxDrawdown   float64       `mapstructure:"max_drawdown"`
	MaxLoss       float64       `mapstructure:"max_loss"`
	PositionLimit float64       `mapstructure:"position_limit"`
	OrderTimeout  time.Duration `mapstructure:"order_timeout"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enable         bool   `mapstructure:"enable"`
	PushGatewayURL string `mapstructure:"push_gateway_url"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enable      bool          `mapstructure:"enable"`
	Channels    []string      `mapstructure:"channels"`
	WebhookURL  string        `mapstructure:"webhook_url"`
	DedupWindow time.Duration `mapstructure:"dedup_window"`
	MinInterval time.Duration `mapstructure:"min_interval"`
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Enable              bool                          `mapstructure:"enable"`
	Name                string                        `mapstructure:"name"`
	Params              map[string]interface{}        `mapstructure:"params"`
	DeltaNeutral        DeltaNeutralStrategyConfig    `mapstructure:"delta_neutral"`
	SmartFilter         SmartFilterConfig             `mapstructure:"smart_filter"`
	TrendFollowing      TrendFollowingStrategyConfig  `mapstructure:"trend_following"`
	MeanReversion       MeanReversionStrategyConfig   `mapstructure:"mean_reversion"`
	VolatilityBreakout  VolatilityBreakoutStrategyConfig `mapstructure:"volatility_breakout"`
}

// SmartFilterConfig SmartFilter配置
type SmartFilterConfig struct {
	Source      string              `mapstructure:"source"`
	CryptoQuant CryptoQuantConfig   `mapstructure:"cryptoquant"`
}

// CryptoQuantConfig CryptoQuant配置
type CryptoQuantConfig struct {
	APIKey  string   `mapstructure:"api_key"`
	Asset   string   `mapstructure:"asset"`
	Assets  []string `mapstructure:"assets"`
}

type DeltaNeutralStrategyConfig struct {
	FundUsagePercent       float64       `mapstructure:"fund_usage_percent"`
	RebalanceThreshold     float64       `mapstructure:"rebalance_threshold"`
	BasisCircuitBreaker    float64       `mapstructure:"basis_circuit_breaker"`
	TargetHedgeRatio       float64       `mapstructure:"target_hedge_ratio"`
	HedgeRatioTolerance    float64       `mapstructure:"hedge_ratio_tolerance"`
	DailyLossLimit         float64       `mapstructure:"daily_loss_limit"`
	MarginBufferPercent    float64       `mapstructure:"margin_buffer_percent"`
	SettlementWindowBefore time.Duration `mapstructure:"settlement_window_before"`
	SettlementWindowAfter  time.Duration `mapstructure:"settlement_window_after"`
}

// TrendFollowingStrategyConfig 趋势跟踪策略配置
type TrendFollowingStrategyConfig struct {
	EMAShortPeriod     int     `mapstructure:"ema_short_period"`
	EMALongPeriod      int     `mapstructure:"ema_long_period"`
	ADXPeriod          int     `mapstructure:"adx_period"`
	ADXThreshold       float64 `mapstructure:"adx_threshold"`
	TrendStrength      float64 `mapstructure:"trend_strength"`
	StopLossPercent    float64 `mapstructure:"stop_loss_percent"`
	TrailingStopPercent float64 `mapstructure:"trailing_stop_percent"`
	SignalCooldown     int64   `mapstructure:"signal_cooldown"`
}

// MeanReversionStrategyConfig 均值回归策略配置
type MeanReversionStrategyConfig struct {
	RSIPeriod          int     `mapstructure:"rsi_period"`
	RSIOverbought      float64 `mapstructure:"rsi_overbought"`
	RSIOversold        float64 `mapstructure:"rsi_oversold"`
	BBPeriod           int     `mapstructure:"bb_period"`
	BBStdDev           float64 `mapstructure:"bb_std_dev"`
	Threshold          float64 `mapstructure:"threshold"`
	StopLossPercent    float64 `mapstructure:"stop_loss_percent"`
	TrailingStopPercent float64 `mapstructure:"trailing_stop_percent"`
	SignalCooldown     int64   `mapstructure:"signal_cooldown"`
}

// VolatilityBreakoutStrategyConfig 波动率突破策略配置
type VolatilityBreakoutStrategyConfig struct {
	ATRPeriod           int     `mapstructure:"atr_period"`
	VolumeMAPeriod      int     `mapstructure:"volume_ma_period"`
	BreakoutMultiplier  float64 `mapstructure:"breakout_multiplier"`
	MinVolumeRatio      float64 `mapstructure:"min_volume_ratio"`
	MaxHoldingBars      int     `mapstructure:"max_holding_bars"`
	StopLossPercent     float64 `mapstructure:"stop_loss_percent"`
	TrailingStopPercent float64 `mapstructure:"trailing_stop_percent"`
	SignalCooldown      int64   `mapstructure:"signal_cooldown"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Enable       bool              `mapstructure:"enable"`
	Port         int               `mapstructure:"port"`
	Host         string            `mapstructure:"host"`
	APIToken     string            `mapstructure:"api_token"`
	TrustedProxies []string        `mapstructure:"trusted_proxies"`
	ForceToken   bool              `mapstructure:"force_token"`
}

// Load 加载配置
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 自动环境变量覆盖
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 解析配置到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 处理环境变量占位符
	if err := resolveEnvVars(&config); err != nil {
		return nil, err
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Validate 对外暴露配置校验，用于 API 写入配置时复用同一套规则。
func Validate(config *Config) error {
	return validateConfig(config)
}

// resolveEnvVars 解析环境变量占位符
func resolveEnvVars(config *Config) error {
	// 解析OKX交易所配置中的环境变量
	config.Exchange.OKX.APIKey = resolveEnvVar(config.Exchange.OKX.APIKey)
	config.Exchange.OKX.SecretKey = resolveEnvVar(config.Exchange.OKX.SecretKey)
	config.Exchange.OKX.Passphrase = resolveEnvVar(config.Exchange.OKX.Passphrase)
	config.Monitoring.Alert.WebhookURL = resolveEnvVar(config.Monitoring.Alert.WebhookURL)

	// 解析LLM配置中的环境变量
	config.LLM.Providers.OpenAI.APIKey = resolveEnvVar(config.LLM.Providers.OpenAI.APIKey)
	config.LLM.Providers.Claude.APIKey = resolveEnvVar(config.LLM.Providers.Claude.APIKey)
	config.LLM.Providers.Qwen.APIKey = resolveEnvVar(config.LLM.Providers.Qwen.APIKey)

	return nil
}

// resolveEnvVar 解析单个环境变量占位符
func resolveEnvVar(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envKey := strings.TrimPrefix(strings.TrimSuffix(value, "}"), "${")
		if envValue := os.Getenv(envKey); envValue != "" {
			return envValue
		}
	}
	return value
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	// 验证基本配置
	if config.Basic.AppName == "" {
		return fmt.Errorf("应用名称不能为空")
	}

	// 验证 OKX 交易所配置
	if config.Exchange.OKX.APIKey == "" {
		return fmt.Errorf("OKX API Key 不能为空")
	}

	if config.Exchange.OKX.SecretKey == "" {
		return fmt.Errorf("OKX API Secret 不能为空")
	}

	if config.Exchange.OKX.Passphrase == "" {
		return fmt.Errorf("OKX Passphrase 不能为空")
	}

	if config.Exchange.OKX.BaseURL == "" {
		return fmt.Errorf("OKX Base URL 不能为空")
	}

	if config.Exchange.OKX.WSURL == "" {
		return fmt.Errorf("OKX WebSocket URL 不能为空")
	}

	// 验证超时配置
	if config.Exchange.OKX.Timeout < 1*time.Second {
		return fmt.Errorf("超时时间不能小于 1 秒")
	}
	if config.Exchange.OKX.Timeout > 60*time.Second {
		return fmt.Errorf("超时时间不能大于 60 秒")
	}

	// 验证重试次数
	if config.Exchange.OKX.RetryCount < 0 || config.Exchange.OKX.RetryCount > 10 {
		return fmt.Errorf("重试次数必须在 0-10 之间")
	}

	// 验证风险配置
	if config.Risk.Enable {
		if config.Risk.MaxPositionSize <= 0 {
			return fmt.Errorf("最大仓位大小必须大于 0")
		}

		if config.Risk.MaxDailyLoss < 0 {
			return fmt.Errorf("最大日亏损不能为负数")
		}

		if config.Risk.MaxDrawdown < 0 || config.Risk.MaxDrawdown > 1 {
			return fmt.Errorf("最大回撤必须在 0 到 1 之间")
		}

		if config.Risk.StopLossPercent < 0 || config.Risk.StopLossPercent > 1 {
			return fmt.Errorf("止损百分比必须在 0 到 1 之间")
		}

		if config.Risk.TakeProfitPercent < 0 || config.Risk.TakeProfitPercent > 1 {
			return fmt.Errorf("止盈百分比必须在 0 到 1 之间")
		}

		if config.Risk.MaxTradesPerDay < 0 {
			return fmt.Errorf("每日最大交易次数不能为负数")
		}

		// 添加合理上限检查
		if config.Risk.MaxTradesPerDay > 10000 {
			return fmt.Errorf("每日最大交易次数不能超过 10000")
		}
	}

	if config.Execution.SmartRouting.OrderBookDepth < 0 {
		return fmt.Errorf("订单簿深度档位不能为负数")
	}
	if config.Execution.Persistence.Enabled {
		if strings.TrimSpace(config.Execution.Persistence.Directory) == "" {
			return fmt.Errorf("执行态持久化目录不能为空")
		}
		if config.Execution.Persistence.SnapshotInterval < time.Second {
			return fmt.Errorf("执行态快照间隔不能小于 1 秒")
		}
	}
	if config.Execution.SmartRouting.MaxEstimatedSlippage < 0 || config.Execution.SmartRouting.MaxEstimatedSlippage > 1 {
		return fmt.Errorf("最大预估滑点必须在 0 到 1 之间")
	}
	if config.Execution.Rebalance.DriftThreshold < 0 || config.Execution.Rebalance.DriftThreshold > 1 {
		return fmt.Errorf("再平衡漂移阈值必须在 0 到 1 之间")
	}
	if config.Execution.Rebalance.MaxPositionsPerCycle < 0 {
		return fmt.Errorf("每轮再平衡最大持仓调整数不能为负数")
	}
	if config.Execution.Rebalance.CircuitCooldown < 0 {
		return fmt.Errorf("再平衡熔断冷却时间不能为负数")
	}
	if config.Execution.Allocator.MinWeight < 0 || config.Execution.Allocator.MinWeight > 1 {
		return fmt.Errorf("分配器最小权重必须在 0 到 1 之间")
	}
	if config.Execution.Allocator.MaxWeight < 0 || config.Execution.Allocator.MaxWeight > 1 {
		return fmt.Errorf("分配器最大权重必须在 0 到 1 之间")
	}
	if config.Execution.Allocator.MinWeight > 0 && config.Execution.Allocator.MaxWeight > 0 && config.Execution.Allocator.MinWeight > config.Execution.Allocator.MaxWeight {
		return fmt.Errorf("分配器最小权重不能大于最大权重")
	}
	if config.Execution.Allocator.WeightChangeThreshold < 0 || config.Execution.Allocator.WeightChangeThreshold > 1 {
		return fmt.Errorf("权重变化阈值必须在 0 到 1 之间")
	}
	if config.Execution.Allocator.PortfolioLossLimit < 0 || config.Execution.Allocator.PortfolioLossLimit > 1 {
		return fmt.Errorf("组合亏损限制必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.FundUsagePercent < 0 || config.Strategy.DeltaNeutral.FundUsagePercent > 1 {
		return fmt.Errorf("DeltaNeutral 资金使用比例必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.RebalanceThreshold < 0 || config.Strategy.DeltaNeutral.RebalanceThreshold > 1 {
		return fmt.Errorf("DeltaNeutral 再平衡阈值必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.BasisCircuitBreaker < 0 || config.Strategy.DeltaNeutral.BasisCircuitBreaker > 1 {
		return fmt.Errorf("DeltaNeutral 基差熔断阈值必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.TargetHedgeRatio < 0 {
		return fmt.Errorf("DeltaNeutral 目标对冲比率不能为负数")
	}
	if config.Strategy.DeltaNeutral.HedgeRatioTolerance < 0 || config.Strategy.DeltaNeutral.HedgeRatioTolerance > 1 {
		return fmt.Errorf("DeltaNeutral 对冲比率容差必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.DailyLossLimit < 0 || config.Strategy.DeltaNeutral.DailyLossLimit > 1 {
		return fmt.Errorf("DeltaNeutral 每日亏损限制必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.MarginBufferPercent < 0 || config.Strategy.DeltaNeutral.MarginBufferPercent > 1 {
		return fmt.Errorf("DeltaNeutral 保证金缓冲比例必须在 0 到 1 之间")
	}
	if config.Strategy.DeltaNeutral.SettlementWindowBefore < 0 {
		return fmt.Errorf("DeltaNeutral 结算前窗口不能为负数")
	}
	if config.Strategy.DeltaNeutral.SettlementWindowAfter < 0 {
		return fmt.Errorf("DeltaNeutral 结算后窗口不能为负数")
	}
	if config.Monitoring.Alert.DedupWindow < 0 {
		return fmt.Errorf("告警去重窗口不能为负数")
	}
	if config.Monitoring.Alert.MinInterval < 0 {
		return fmt.Errorf("告警最小发送间隔不能为负数")
	}

	// 验证回测配置
	if config.Backtest.Enable {
		if config.Backtest.InitialBalance <= 0 {
			return fmt.Errorf("初始余额必须大于 0")
		}
	}

	// 验证服务器配置
	if config.Server.Enable {
		if config.Server.Port <= 0 || config.Server.Port > 65535 {
			return fmt.Errorf("服务器端口必须在 1 到 65535 之间")
		}

		if config.Server.Host == "" {
			return fmt.Errorf("服务器主机不能为空")
		}
	}

	// 验证数据库配置
	if config.Database.Enable {
		if config.Database.Type == "" {
			return fmt.Errorf("数据库类型不能为空")
		}
		if config.Database.Path == "" {
			return fmt.Errorf("数据库路径不能为空")
		}
	}

	// 验证LLM配置
	if config.LLM.Enable {
		if config.LLM.Provider == "" {
			return fmt.Errorf("LLM提供商不能为空")
		}
		if config.LLM.Timeout < 1*time.Second {
			return fmt.Errorf("LLM超时时间不能小于1秒")
		}
		if config.LLM.Timeout > 120*time.Second {
			return fmt.Errorf("LLM超时时间不能大于120秒")
		}
	}

	// 验证提醒服务配置
	if config.Alert.Enable {
		if config.Alert.PriceChangeThreshold < 0 || config.Alert.PriceChangeThreshold > 1 {
			return fmt.Errorf("价格变化阈值必须在0到1之间")
		}
		if config.Alert.CheckInterval < 1*time.Second {
			return fmt.Errorf("检查间隔不能小于1秒")
		}
	}

	return nil
}
