package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/alertservice"
	"github.com/ljwqf/quant/internal/backtest"
	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/dataservice"
	"github.com/ljwqf/quant/internal/indicator"
	"github.com/ljwqf/quant/internal/llmanalysis"
	"github.com/ljwqf/quant/internal/manualtrading"
	"github.com/ljwqf/quant/internal/monitoring"
	"github.com/ljwqf/quant/internal/notifications"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/strategy"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v3"
)

const maskedSecretValue = "******"

const maxRecentRebalanceEvents = 50

// ActionHandlers 提供 API 管理接口需要调用的业务动作。
type ActionHandlers struct {
	StartStrategy         func(name string) (*StrategyStatus, error)
	StopStrategy          func(name string) (*StrategyStatus, error)
	CreateOrder           func(order *types.Order) (*types.OrderResult, error)
	ClosePosition         func(symbol string) (*types.OrderResult, error)
	GetRebalanceCircuit   func() (*RebalanceCircuitInfo, error)
	ResetRebalanceCircuit func(reason string) (*RebalanceCircuitInfo, error)
	GetTicker             func(symbol string) (*types.Tick, error)
	GetBars               func(symbol string, interval string, limit int) ([]*types.Bar, error)
	GetOrderBook          func(symbol string, depth int) (*types.OrderBook, error)
}

type createOrderRequest struct {
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Type   string  `json:"type"`
	Price  float64 `json:"price"`
	Size   float64 `json:"size"`
}

type resetRebalanceCircuitRequest struct {
	Reason string `json:"reason"`
}

// Server API服务器
type Server struct {
	port            int
	host            string
	server          *http.Server
	mux             *http.ServeMux
	wsHub           *WebSocketHub
	mutex           sync.RWMutex
	configPath      string
	cfg             *config.Config
	apiToken        string
	trustedProxies  []*net.IPNet
	forceToken      bool
	tlsEnable       bool
	tlsCertFile     string
	tlsKeyFile      string
	ipWhitelist     *IPWhitelist
	actions         *ActionHandlers
	manualTradeMgr  *manualtrading.Manager
	analyzer        *llmanalysis.Analyzer
	dataService     *dataservice.DataService
	alertService    *alertservice.AlertService
	notificationMgr *notifications.NotificationManager
	indicatorSet    *indicator.IndicatorSet
	metrics         *monitoring.Metrics
	strategyEngine  *strategy.Engine

	// 速率限制
	rateLimitMu      sync.Mutex
	rateLimitBuckets map[string]*rateBucket

	// 系统状态
	systemStatus *SystemStatus
	strategies   map[string]*StrategyStatus
	positions    []*PositionInfo
	orders       []*OrderInfo
	signals      []*SignalInfo
	rebalance    []*RebalanceEventInfo
}

type rateBucket struct {
	tokens    int
	lastReset time.Time
}

// 手动交易相关请求结构
type createManualOrderRequest struct {
	Symbol            string  `json:"symbol"`
	Side              string  `json:"side"`
	Type              string  `json:"type"`
	Price             float64 `json:"price,omitempty"`
	Size              float64 `json:"size"`
	Leverage          int     `json:"leverage,omitempty"`
	TakeProfit        float64 `json:"take_profit,omitempty"`
	StopLoss          float64 `json:"stop_loss,omitempty"`
	AIAnalysisID      int64   `json:"ai_analysis_id,omitempty"`
	AIAnalysisSummary string  `json:"ai_analysis_summary,omitempty"`
}

type closePositionRequest struct {
	Symbol string  `json:"symbol"`
	Size   float64 `json:"size,omitempty"`
}

type setTpSlRequest struct {
	Symbol     string  `json:"symbol"`
	TakeProfit float64 `json:"take_profit,omitempty"`
	StopLoss   float64 `json:"stop_loss,omitempty"`
}

type setLeverageRequest struct {
	Symbol     string `json:"symbol"`
	Leverage   int    `json:"leverage"`
	MarginMode string `json:"margin_mode,omitempty"`
}

type setTrailingStopRequest struct {
	Symbol       string  `json:"symbol"`
	StopDistance float64 `json:"stop_distance"`
}

type createTimedOrderRequest struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Size      float64 `json:"size"`
	ExecuteAt string  `json:"execute_at"`
}

type createConditionalOrderRequest struct {
	Symbol          string                 `json:"symbol"`
	Side            string                 `json:"side"`
	Size            float64                `json:"size"`
	OrderType       string                 `json:"order_type"`
	ConditionalType string                 `json:"conditional_type"`
	Condition       map[string]interface{} `json:"condition"`
	Price           float64                `json:"price,omitempty"`
}

// SystemStatus 系统状态
type SystemStatus struct {
	Running           bool      `json:"running"`
	ExchangeConnected bool      `json:"exchange_connected"`
	StartTime         time.Time `json:"start_time"`
	Uptime            string    `json:"uptime"`
	AccountBalance    float64   `json:"account_balance"`
	TotalPnL          float64   `json:"total_pnl"`
	DailyPnL          float64   `json:"daily_pnl"`
	WinRate           float64   `json:"win_rate"`
	TotalTrades       int       `json:"total_trades"`
}

// StrategyStatus 策略状态
type StrategyStatus struct {
	Name       string    `json:"name"`
	Enabled    bool      `json:"enabled"`
	Running    bool      `json:"running"`
	PnL        float64   `json:"pnl"`
	WinRate    float64   `json:"win_rate"`
	Trades     int       `json:"trades"`
	LastSignal string    `json:"last_signal"`
	LastUpdate time.Time `json:"last_update"`
}

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`
	Size          float64   `json:"size"`
	EntryPrice    float64   `json:"entry_price"`
	MarkPrice     float64   `json:"mark_price"`
	UnrealizedPnL float64   `json:"unrealized_pnl"`
	Leverage      int       `json:"leverage"`
	OpenTime      time.Time `json:"open_time"`
	Strategy      string    `json:"strategy"`
}

// OrderInfo 订单信息
type OrderInfo struct {
	OrderID    string    `json:"order_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Type       string    `json:"type"`
	Price      float64   `json:"price"`
	Size       float64   `json:"size"`
	FilledSize float64   `json:"filled_size"`
	Status     string    `json:"status"`
	CreateTime time.Time `json:"create_time"`
	Strategy   string    `json:"strategy"`
}

// SignalInfo 信号信息
type SignalInfo struct {
	ID         string    `json:"id"`
	Strategy   string    `json:"strategy"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Price      float64   `json:"price"`
	Size       float64   `json:"size"`
	Confidence float64   `json:"confidence"`
	Reason     string    `json:"reason"`
	Time       time.Time `json:"time"`
	Executed   bool      `json:"executed"`
}

type RebalanceCircuitInfo struct {
	Open            bool      `json:"open"`
	Strategy        string    `json:"strategy"`
	Step            string    `json:"step"`
	Reason          string    `json:"reason"`
	OpenedAt        time.Time `json:"opened_at"`
	CooldownUntil   time.Time `json:"cooldown_until"`
	LastResetAt     time.Time `json:"last_reset_at"`
	LastResetReason string    `json:"last_reset_reason"`
	AutoReset       bool      `json:"auto_reset"`
	Cooldown        string    `json:"cooldown"`
}

type RebalanceCircuitResetEvent struct {
	Success   bool                  `json:"success"`
	Message   string                `json:"message"`
	Reason    string                `json:"reason"`
	Circuit   *RebalanceCircuitInfo `json:"circuit,omitempty"`
	Timestamp time.Time             `json:"timestamp"`
}

type RebalanceEventInfo struct {
	Type      string                 `json:"type"`
	Strategy  string                 `json:"strategy"`
	Step      string                 `json:"step"`
	Reason    string                 `json:"reason"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Circuit   *RebalanceCircuitInfo  `json:"circuit,omitempty"`
}

// NewServer 创建API服务器
func NewServer(host string, port int, cfg *config.Config, configPath string, actions *ActionHandlers) *Server {
	if host == "" {
		host = "127.0.0.1"
	}

	s := &Server{
		port:       port,
		host:       host,
		mux:        http.NewServeMux(),
		configPath: configPath,
		cfg:        cfg,
		apiToken:   strings.TrimSpace(os.Getenv("OKX_QUANT_API_TOKEN")),
		actions:    actions,
		strategies: make(map[string]*StrategyStatus),
		positions:  make([]*PositionInfo, 0),
		orders:     make([]*OrderInfo, 0),
		signals:    make([]*SignalInfo, 0),
		rebalance:  make([]*RebalanceEventInfo, 0, maxRecentRebalanceEvents),
		systemStatus: &SystemStatus{
			StartTime: time.Now(),
		},
	}

	if cfg != nil {
		s.forceToken = cfg.Server.ForceToken
		s.trustedProxies = parseTrustedProxies(cfg.Server.TrustedProxies)
		if cfg.Server.APIToken != "" {
			s.apiToken = cfg.Server.APIToken
		}
		s.tlsEnable = cfg.Server.TLSEnable
		s.tlsCertFile = cfg.Server.TLSCertFile
		s.tlsKeyFile = cfg.Server.TLSKeyFile
		s.ipWhitelist = NewIPWhitelist(cfg.Server.IPWhitelist, func(ip string) {
			logger.Info("IP白名单拦截", zap.String("ip", ip))
		})
	}

	s.wsHub = NewWebSocketHub(s)
	s.setupRoutes()

	return s
}

// SetManualTradeManager 设置手动交易管理器
func (s *Server) SetManualTradeManager(mgr *manualtrading.Manager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.manualTradeMgr = mgr
}

// SetAnalyzer 设置大模型分析器
func (s *Server) SetAnalyzer(analyzer *llmanalysis.Analyzer) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.analyzer = analyzer
}

// SetDataService 设置数据采集服务
func (s *Server) SetDataService(dataService *dataservice.DataService) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.dataService = dataService
}

// SetAlertService 设置提醒服务
func (s *Server) SetAlertService(alertService *alertservice.AlertService) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.alertService = alertService
}

// SetNotificationManager 设置通知管理器
func (s *Server) SetNotificationManager(notificationMgr *notifications.NotificationManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.notificationMgr = notificationMgr
}

// SetIndicatorSet 设置技术指标集
func (s *Server) SetIndicatorSet(indicatorSet *indicator.IndicatorSet) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.indicatorSet = indicatorSet
}

// GetWebSocketHub 获取WebSocket Hub，用于注册WebSocket通道
func (s *Server) GetWebSocketHub() *WebSocketHub {
	return s.wsHub
}

// GetMux 获取HTTP ServeMux，用于测试
func (s *Server) GetMux() *http.ServeMux {
	return s.mux
}

// 大模型分析相关请求结构
type analyzeTradeRequest struct {
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Size   float64 `json:"size"`
	Price  float64 `json:"price,omitempty"`
}

type analyzeMarketRequest struct {
	Symbol string `json:"symbol"`
}

func parseTrustedProxies(proxies []string) []*net.IPNet {
	var networks []*net.IPNet
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if !strings.Contains(proxy, "/") {
			proxy = proxy + "/32"
		}
		_, ipNet, err := net.ParseCIDR(proxy)
		if err != nil {
			logger.Warn("无效的可信代理IP配置", zap.String("proxy", proxy), zap.Error(err))
			continue
		}
		networks = append(networks, ipNet)
	}
	return networks
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	fs := http.FileServer(http.Dir("./web"))
	s.mux.Handle("/", fs)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))

	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/ready", s.handleReady)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/strategies", s.handleStrategies)
	s.mux.HandleFunc("/api/positions", s.handlePositions)
	s.mux.HandleFunc("/api/orders", s.handleOrders)
	s.mux.HandleFunc("/api/signals", s.handleSignals)
	s.mux.HandleFunc("/api/account", s.handleAccount)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/strategy/start/", s.handleStrategyStart)
	s.mux.HandleFunc("/api/strategy/stop/", s.handleStrategyStop)
	s.mux.HandleFunc("/api/strategy/params/", s.handleStrategyParams)
	s.mux.HandleFunc("/api/order/create", s.handleCreateOrder)
	s.mux.HandleFunc("/api/position/close/", s.handleClosePosition)
	s.mux.HandleFunc("/api/rebalance/circuit", s.handleRebalanceCircuit)
	s.mux.HandleFunc("/api/rebalance/events", s.handleRebalanceEvents)
	s.mux.HandleFunc("/api/rebalance/circuit/reset", s.handleRebalanceCircuitReset)
	s.mux.HandleFunc("/ws", s.wsHub.HandleWebSocket)

	// 手动交易API路由
	s.mux.HandleFunc("/api/manual/order", s.handleManualCreateOrder)
	s.mux.HandleFunc("/api/manual/order/", s.handleManualCancelOrder)
	s.mux.HandleFunc("/api/manual/orders", s.handleManualListOrders)
	s.mux.HandleFunc("/api/manual/position/close", s.handleManualClosePosition)
	s.mux.HandleFunc("/api/manual/position/tp-sl", s.handleManualSetTpSl)
	s.mux.HandleFunc("/api/manual/position/leverage", s.handleManualSetLeverage)
	s.mux.HandleFunc("/api/manual/position/trailing-stop", s.handleManualSetTrailingStop)

	// 限时单API路由
	s.mux.HandleFunc("/api/manual/timed-order", s.handleCreateTimedOrder)
	s.mux.HandleFunc("/api/manual/timed-order/", s.handleCancelTimedOrder)
	s.mux.HandleFunc("/api/manual/timed-orders", s.handleListTimedOrders)

	// 条件单API路由
	s.mux.HandleFunc("/api/manual/conditional-order", s.handleCreateConditionalOrder)
	s.mux.HandleFunc("/api/manual/conditional-order/", s.handleCancelConditionalOrder)
	s.mux.HandleFunc("/api/manual/conditional-orders", s.handleListConditionalOrders)

	// 市场数据API路由
	s.mux.HandleFunc("/api/market/ticker", s.handleGetTicker)
	s.mux.HandleFunc("/api/market/bars", s.handleGetBars)
	s.mux.HandleFunc("/api/market/orderbook", s.handleGetOrderBook)

	// 大模型分析API路由
	s.mux.HandleFunc("/api/llm/analyze/trade", s.handleLLMAnalyzeTrade)
	s.mux.HandleFunc("/api/llm/analyze/positions", s.handleLLMAnalyzePositions)
	s.mux.HandleFunc("/api/llm/analyze/market", s.handleLLMAnalyzeMarket)
	s.mux.HandleFunc("/api/llm/analyze/orders", s.handleLLMAnalyzeOrders)
	s.mux.HandleFunc("/api/llm/history", s.handleLLMHistory)

	// 数据采集服务API路由
	s.mux.HandleFunc("/api/data/news", s.handleGetNews)
	s.mux.HandleFunc("/api/data/events", s.handleGetEvents)
	s.mux.HandleFunc("/api/data/collect", s.handleCollectNow)
	s.mux.HandleFunc("/api/data/history", s.handleGetHistoryData)
	s.mux.HandleFunc("/api/data/ticks", s.handleGetTickData)

	// 提醒服务API路由
	s.mux.HandleFunc("/api/alerts", s.handleGetAlerts)
	s.mux.HandleFunc("/api/alerts/send", s.handleSendAlert)

	// 技术指标API路由
	s.mux.HandleFunc("/api/indicators/calculate", s.handleCalculateIndicators)
	s.mux.HandleFunc("/api/indicators/list", s.handleListIndicators)

	// 回测API路由
	s.mux.HandleFunc("/api/backtest/start", s.handleBacktestStart)
	s.mux.HandleFunc("/api/backtest/strategies", s.handleBacktestStrategies)
	s.mux.HandleFunc("/api/backtest/results/", s.handleBacktestGetResults)
	s.mux.HandleFunc("/api/backtest/report/", s.handleBacktestGetReport)

	// 监控API路由
	s.mux.HandleFunc("/api/metrics", s.handleGetMetrics)
	s.mux.HandleFunc("/api/metrics/prometheus", s.handlePrometheusMetrics)

	// Swagger UI
	s.mux.Handle("/swagger/", httpSwagger.WrapHandler)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	ready := true
	checks := make(map[string]string)

	if s.systemStatus == nil {
		ready = false
		checks["system_status"] = "not initialized"
	} else if !s.systemStatus.Running {
		ready = false
		checks["system_status"] = "not running"
	} else {
		checks["system_status"] = "ok"
	}

	if !s.systemStatus.ExchangeConnected {
		ready = false
		checks["exchange"] = "disconnected"
	} else {
		checks["exchange"] = "connected"
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}
	writeJSON(w, statusCode, map[string]interface{}{
		"ready":  ready,
		"checks": checks,
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	// 将 mux 包装为带中间件的 handler
	handler := corsMiddleware(securityHeadersMiddleware(s.mux))
	if s.ipWhitelist != nil {
		handler = s.ipWhitelist.Middleware(handler)
	}

	server := &http.Server{
		Addr:         net.JoinHostPort(s.host, strconv.Itoa(s.port)),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,  // LLM 分析可能耗时较长
		IdleTimeout:  120 * time.Second,
	}

	s.mutex.Lock()
	s.server = server
	s.systemStatus.Running = true
	s.systemStatus.StartTime = time.Now()
	s.mutex.Unlock()

	go s.wsHub.Run()

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	logger.Info("API服务器启动", zap.String("addr", addr), zap.Bool("tls", s.tlsEnable))
	if s.tlsEnable && s.tlsCertFile != "" && s.tlsKeyFile != "" {
		return server.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile)
	}
	return server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mutex.Lock()
	s.systemStatus.Running = false
	server := s.server
	s.mutex.Unlock()

	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
	return nil
}

// IsRunning 返回服务器运行状态。
func (s *Server) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.systemStatus == nil {
		return false
	}
	return s.systemStatus.Running
}

// UpdateSystemStatus 更新系统状态
func (s *Server) UpdateSystemStatus(status *SystemStatus) {
	if status == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	status.Uptime = time.Since(s.systemStatus.StartTime).String()
	s.systemStatus = status
	s.wsHub.Broadcast("status", status)
}

// UpdateAccountBalance 更新账户余额（不覆盖其他状态字段）
func (s *Server) UpdateAccountBalance(balance float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.systemStatus != nil {
		s.systemStatus.AccountBalance = balance
		s.wsHub.Broadcast("status", s.systemStatus)
	}
}

// UpdateStrategyStatus 更新策略状态
func (s *Server) UpdateStrategyStatus(name string, status *StrategyStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.strategies[name] = status
	s.wsHub.Broadcast("strategy", status)
}

func (s *Server) UpdateRebalanceCircuit(info *RebalanceCircuitInfo) {
	if info == nil {
		return
	}
	s.wsHub.Broadcast("rebalance_circuit", info)
}

func (s *Server) BroadcastRebalanceCircuitReset(event *RebalanceCircuitResetEvent) {
	if event == nil {
		return
	}
	s.wsHub.Broadcast("rebalance_circuit_reset", event)
}

func (s *Server) BroadcastRebalanceEvent(event *RebalanceEventInfo) {
	if event == nil {
		return
	}
	s.mutex.Lock()
	s.rebalance = append(s.rebalance, cloneRebalanceEventInfo(event))
	if len(s.rebalance) > maxRecentRebalanceEvents {
		s.rebalance = s.rebalance[len(s.rebalance)-maxRecentRebalanceEvents:]
	}
	s.mutex.Unlock()
	s.wsHub.Broadcast("rebalance_event", event)
}

func (s *Server) GetRecentRebalanceEvents(limit int) []*RebalanceEventInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if limit <= 0 || limit > maxRecentRebalanceEvents {
		limit = maxRecentRebalanceEvents
	}
	if len(s.rebalance) == 0 {
		return []*RebalanceEventInfo{}
	}
	start := 0
	if len(s.rebalance) > limit {
		start = len(s.rebalance) - limit
	}
	events := make([]*RebalanceEventInfo, 0, len(s.rebalance)-start)
	for _, event := range s.rebalance[start:] {
		events = append(events, cloneRebalanceEventInfo(event))
	}
	return events
}

// UpdatePositions 更新持仓
func (s *Server) UpdatePositions(positions []*PositionInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.positions = positions
	s.wsHub.Broadcast("positions", positions)
}

// AddSignal 添加信号
func (s *Server) AddSignal(signal *SignalInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.signals = append(s.signals, signal)
	if len(s.signals) > 100 {
		s.signals = s.signals[1:]
	}
	s.wsHub.Broadcast("signal", signal)
}

// AddOrder 添加订单
func (s *Server) AddOrder(order *OrderInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.orders = append(s.orders, order)
	if len(s.orders) > 100 {
		s.orders = s.orders[1:]
	}
	s.wsHub.Broadcast("order", order)
}

// API处理函数
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	writeJSON(w, http.StatusOK, s.systemStatus)
}

func (s *Server) handleStrategies(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	strategies := make([]*StrategyStatus, 0)
	for _, st := range s.strategies {
		strategies = append(strategies, st)
	}
	writeJSON(w, http.StatusOK, strategies)
}

func (s *Server) handlePositions(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	writeJSON(w, http.StatusOK, s.positions)
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	writeJSON(w, http.StatusOK, s.orders)
}

func (s *Server) handleSignals(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	writeJSON(w, http.StatusOK, s.signals)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	writeJSON(w, http.StatusOK, s.systemStatus)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPut, http.MethodPost:
		s.saveConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.cfg == nil {
		http.Error(w, "Config not loaded", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, s.maskedConfigLocked())
}

func (s *Server) saveConfig(w http.ResponseWriter, r *http.Request) {
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	var newConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mutex.RLock()
	currentConfig := s.cfg
	s.mutex.RUnlock()

	mergeProtectedFields(&newConfig, currentConfig)
	if err := config.Validate(&newConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mutex.Lock()
	s.cfg = &newConfig
	s.mutex.Unlock()

	if s.configPath != "" {
		data, err := yaml.Marshal(&newConfig)
		if err != nil {
			logger.Error("序列化配置失败", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(s.configPath, data, 0600); err != nil {
			logger.Error("保存配置文件失败", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		logger.Info("配置已保存", zap.String("path", s.configPath))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "配置已保存，重启后生效",
	})
}

func (s *Server) handleStrategyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}
	if s.actions == nil || s.actions.StartStrategy == nil {
		http.Error(w, "Strategy control is not available", http.StatusNotImplemented)
		return
	}

	strategyName := strings.TrimPrefix(r.URL.Path, "/api/strategy/start/")
	if strategyName == "" {
		http.Error(w, "strategy name is required", http.StatusBadRequest)
		return
	}

	status, err := s.actions.StartStrategy(strategyName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if status != nil {
		s.UpdateStrategyStatus(strategyName, status)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "started", "strategy": strategyName})
}

func (s *Server) handleStrategyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}
	if s.actions == nil || s.actions.StopStrategy == nil {
		http.Error(w, "Strategy control is not available", http.StatusNotImplemented)
		return
	}

	strategyName := strings.TrimPrefix(r.URL.Path, "/api/strategy/stop/")
	if strategyName == "" {
		http.Error(w, "strategy name is required", http.StatusBadRequest)
		return
	}

	status, err := s.actions.StopStrategy(strategyName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if status != nil {
		s.UpdateStrategyStatus(strategyName, status)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "stopped", "strategy": strategyName})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}
	if s.actions == nil || s.actions.CreateOrder == nil {
		http.Error(w, "Order creation is not available", http.StatusNotImplemented)
		return
	}

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order, err := buildOrderFromRequest(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.actions.CreateOrder(order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if result != nil {
		s.AddOrder(&OrderInfo{
			OrderID:    result.OrderID,
			Symbol:     result.Symbol,
			Side:       string(result.Side),
			Type:       string(result.Type),
			Price:      result.Price,
			Size:       result.Quantity,
			FilledSize: result.Quantity,
			Status:     string(result.Status),
			CreateTime: result.Timestamp,
			Strategy:   "manual",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "created", "result": result})
}

func (s *Server) handleClosePosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}
	if s.actions == nil || s.actions.ClosePosition == nil {
		http.Error(w, "Position close is not available", http.StatusNotImplemented)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/position/close/")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	result, err := s.actions.ClosePosition(symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "closed", "result": result})
}

func (s *Server) handleRebalanceCircuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}
	if s.actions == nil || s.actions.GetRebalanceCircuit == nil {
		http.Error(w, "Rebalance circuit control is not available", http.StatusNotImplemented)
		return
	}
	state, err := s.actions.GetRebalanceCircuit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleRebalanceEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.GetRecentRebalanceEvents(maxRecentRebalanceEvents))
}

func (s *Server) handleRebalanceCircuitReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}
	if s.actions == nil || s.actions.ResetRebalanceCircuit == nil {
		http.Error(w, "Rebalance circuit control is not available", http.StatusNotImplemented)
		return
	}
	var req resetRebalanceCircuitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resetReason := strings.TrimSpace(req.Reason)
	state, err := s.actions.ResetRebalanceCircuit(resetReason)
	if err != nil {
		currentState := (*RebalanceCircuitInfo)(nil)
		if s.actions.GetRebalanceCircuit != nil {
			state, stateErr := s.actions.GetRebalanceCircuit()
			if stateErr != nil {
				logger.Warn("获取重置失败后的熔断状态失败", zap.Error(stateErr))
			} else {
				currentState = state
			}
		}
		s.BroadcastRebalanceCircuitReset(&RebalanceCircuitResetEvent{
			Success:   false,
			Message:   err.Error(),
			Reason:    resetReason,
			Circuit:   currentState,
			Timestamp: time.Now(),
		})
		if currentState != nil {
			s.UpdateRebalanceCircuit(currentState)
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.UpdateRebalanceCircuit(state)
	s.BroadcastRebalanceCircuitReset(&RebalanceCircuitResetEvent{
		Success:   true,
		Message:   "rebalance circuit reset",
		Reason:    resetReason,
		Circuit:   state,
		Timestamp: time.Now(),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "reset", "circuit": state})
}

func buildOrderFromRequest(req *createOrderRequest) (*types.Order, error) {
	if req == nil {
		return nil, fmt.Errorf("request body is required")
	}
	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if req.Size <= 0 {
		return nil, fmt.Errorf("size must be greater than 0")
	}

	orderSide := types.OrderSide(strings.ToLower(req.Side))
	if orderSide != types.OrderSideBuy && orderSide != types.OrderSideSell {
		return nil, fmt.Errorf("invalid order side")
	}

	orderType := types.OrderType(strings.ToLower(req.Type))
	if orderType != types.OrderTypeLimit && orderType != types.OrderTypeMarket {
		return nil, fmt.Errorf("invalid order type")
	}
	if orderType == types.OrderTypeLimit && req.Price <= 0 {
		return nil, fmt.Errorf("limit order price must be greater than 0")
	}

	return &types.Order{
		Symbol:    req.Symbol,
		Side:      orderSide,
		Type:      orderType,
		Price:     req.Price,
		Quantity:  req.Size,
		Leverage:  1,
		Timestamp: time.Now(),
	}, nil
}

func cloneRebalanceEventInfo(event *RebalanceEventInfo) *RebalanceEventInfo {
	if event == nil {
		return nil
	}
	cloned := *event
	if event.Labels != nil {
		cloned.Labels = make(map[string]string, len(event.Labels))
		for key, value := range event.Labels {
			cloned.Labels[key] = value
		}
	}
	if event.Details != nil {
		cloned.Details = make(map[string]interface{}, len(event.Details))
		for key, value := range event.Details {
			cloned.Details[key] = value
		}
	}
	if event.Circuit != nil {
		circuit := *event.Circuit
		cloned.Circuit = &circuit
	}
	return &cloned
}

func (s *Server) maskedConfigLocked() *config.Config {
	if s.cfg == nil {
		return nil
	}

	masked := *s.cfg
	masked.Exchange.OKX.APIKey = maskSecret(masked.Exchange.OKX.APIKey)
	masked.Exchange.OKX.SecretKey = maskSecret(masked.Exchange.OKX.SecretKey)
	masked.Exchange.OKX.Passphrase = maskSecret(masked.Exchange.OKX.Passphrase)
	masked.Monitoring.Alert.WebhookURL = maskSecret(masked.Monitoring.Alert.WebhookURL)
	masked.Notifications.Telegram.BotToken = maskSecret(masked.Notifications.Telegram.BotToken)
	masked.Notifications.Telegram.ChatID = maskSecret(masked.Notifications.Telegram.ChatID)
	masked.Notifications.Discord.WebhookURL = maskSecret(masked.Notifications.Discord.WebhookURL)
	masked.Notifications.Email.Password = maskSecret(masked.Notifications.Email.Password)
	return &masked
}

func mergeProtectedFields(newConfig, currentConfig *config.Config) {
	if newConfig == nil || currentConfig == nil {
		return
	}

	if shouldPreserveSecret(newConfig.Exchange.OKX.APIKey) {
		newConfig.Exchange.OKX.APIKey = currentConfig.Exchange.OKX.APIKey
	}
	if shouldPreserveSecret(newConfig.Exchange.OKX.SecretKey) {
		newConfig.Exchange.OKX.SecretKey = currentConfig.Exchange.OKX.SecretKey
	}
	if shouldPreserveSecret(newConfig.Exchange.OKX.Passphrase) {
		newConfig.Exchange.OKX.Passphrase = currentConfig.Exchange.OKX.Passphrase
	}
	if shouldPreserveSecret(newConfig.Monitoring.Alert.WebhookURL) {
		newConfig.Monitoring.Alert.WebhookURL = currentConfig.Monitoring.Alert.WebhookURL
	}
	if shouldPreserveSecret(newConfig.Notifications.Telegram.BotToken) {
		newConfig.Notifications.Telegram.BotToken = currentConfig.Notifications.Telegram.BotToken
	}
	if shouldPreserveSecret(newConfig.Notifications.Telegram.ChatID) {
		newConfig.Notifications.Telegram.ChatID = currentConfig.Notifications.Telegram.ChatID
	}
	if shouldPreserveSecret(newConfig.Notifications.Discord.WebhookURL) {
		newConfig.Notifications.Discord.WebhookURL = currentConfig.Notifications.Discord.WebhookURL
	}
	if shouldPreserveSecret(newConfig.Notifications.Email.Password) {
		newConfig.Notifications.Email.Password = currentConfig.Notifications.Email.Password
	}
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return maskedSecretValue
}

func shouldPreserveSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == maskedSecretValue
}

func (s *Server) requireMutationAccess(r *http.Request) error {
	if s.hasValidToken(r.Header.Get("X-API-Token")) {
		return nil
	}

	if s.apiToken != "" || s.forceToken {
		return fmt.Errorf("mutation endpoint requires a valid X-API-Token")
	}

	if s.isTrustedRequest(r) {
		return nil
	}

	return fmt.Errorf("mutation endpoint requires trusted access or a valid X-API-Token")
}

// checkRateLimit returns false if the client has exceeded the rate limit.
func (s *Server) checkRateLimit(w http.ResponseWriter, r *http.Request) bool {
	if !s.rateLimit(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "请求频率过高，请稍后重试"})
		return false
	}
	return true
}

func (s *Server) hasValidToken(token string) bool {
	if s.apiToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(s.apiToken)) == 1
}

func isLocalRequest(r *http.Request) bool {
	host := r.RemoteAddr
	// RemoteAddr may include port, extract just the IP
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// Handle IPv6 bracket notation
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func (s *Server) isTrustedRequest(r *http.Request) bool {
	if s.forceToken {
		return false
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return true
	}

	for _, trustedNet := range s.trustedProxies {
		if trustedNet != nil && trustedNet.Contains(ip) {
			xff := r.Header.Get("X-Forwarded-For")
			if xff != "" {
				xffIPs := strings.Split(xff, ",")
				for _, xffIP := range xffIPs {
					xffIP = strings.TrimSpace(xffIP)
					if xffIP != "" {
						parsedXffIP := net.ParseIP(xffIP)
						if parsedXffIP != nil && parsedXffIP.IsLoopback() {
							return true
						}
					}
				}
			}
			return false
		}
	}

	return false
}

func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) bool {
	requireToken := s.apiToken != "" || s.forceToken
	if !requireToken && s.isTrustedRequest(r) {
		return true
	}

	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权：缺少认证令牌"})
		return false
	}

	if !s.hasValidToken(token) {
		logger.Warn("API认证失败",
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("path", r.URL.Path),
		)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权：无效的认证令牌"})
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Warn("写入JSON响应失败", zap.Error(err), zap.Int("status_code", statusCode))
	}
}

// corsMiddleware 跨域资源共享中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware 安全响应头中间件
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		next.ServeHTTP(w, r)
	})
}

// requireReadAccess 读请求认证（所有 /api/* GET 请求）
func (s *Server) requireReadAccess(w http.ResponseWriter, r *http.Request) bool {
	// 未配置 token 则放行（与 requireMutationAccess 保持一致）
	if s.apiToken == "" {
		return true
	}
	// 检查 X-API-Token 头
	token := r.Header.Get("X-API-Token")
	if token == "" {
		// 也支持 Authorization Bearer
		auth := r.Header.Get("Authorization")
		token = strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			token = ""
		}
	}
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权：缺少认证令牌"})
		return false
	}
	if !s.hasValidToken(token) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权：无效的认证令牌"})
		return false
	}
	return true
}

// rateLimiter 简单的基于 IP 的速率限制（每分钟 60 次突变请求）
func (s *Server) rateLimit(r *http.Request) bool {
	if s.apiToken == "" {
		return true // 无 token 则不限制
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()
	if s.rateLimitBuckets == nil {
		s.rateLimitBuckets = make(map[string]*rateBucket)
	}
	bucket, exists := s.rateLimitBuckets[ip]
	if !exists || time.Since(bucket.lastReset) > time.Minute {
		s.rateLimitBuckets[ip] = &rateBucket{tokens: 60, lastReset: time.Now()}
		return true
	}
	if bucket.tokens <= 0 {
		return false
	}
	bucket.tokens--
	return true
}

// 手动交易处理函数
func (s *Server) handleManualCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req createManualOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}
	if req.Side == "" {
		http.Error(w, "side is required", http.StatusBadRequest)
		return
	}
	if req.Size <= 0 {
		http.Error(w, "size must be greater than 0", http.StatusBadRequest)
		return
	}

	trade := &storage.ManualTrade{
		Symbol:            req.Symbol,
		Side:              req.Side,
		Type:              req.Type,
		Price:             req.Price,
		Size:              req.Size,
		Leverage:          req.Leverage,
		TakeProfit:        req.TakeProfit,
		StopLoss:          req.StopLoss,
		AIAnalysisID:      req.AIAnalysisID,
		AIAnalysisSummary: req.AIAnalysisSummary,
	}

	if err := mgr.OrderManager().CreateOrder(trade); err != nil {
		logger.Error("创建手动交易订单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"trade":  trade,
	})
}

func (s *Server) handleManualCancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/api/manual/order/")
	if orderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	if err := mgr.OrderManager().CancelOrder(orderID); err != nil {
		logger.Error("撤销手动交易订单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"order_id": orderID,
	})
}

func (s *Server) handleManualListOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	symbol := r.URL.Query().Get("symbol")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	orders, err := mgr.OrderManager().ListOrders(symbol, limit, offset)
	if err != nil {
		logger.Error("获取手动交易订单列表失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"orders": orders,
	})
}

func (s *Server) handleManualClosePosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req closePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if err := mgr.PositionManager().ClosePosition(req.Symbol, req.Size); err != nil {
		logger.Error("平仓失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"symbol": req.Symbol,
	})
}

func (s *Server) handleManualSetTpSl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req setTpSlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if err := mgr.PositionManager().SetTakeProfitStopLoss(req.Symbol, req.TakeProfit, req.StopLoss); err != nil {
		logger.Error("设置止盈止损失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"symbol":      req.Symbol,
		"take_profit": req.TakeProfit,
		"stop_loss":   req.StopLoss,
	})
}

func (s *Server) handleManualSetLeverage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req setLeverageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.Leverage <= 0 {
		http.Error(w, "leverage must be greater than 0", http.StatusBadRequest)
		return
	}

	if err := mgr.PositionManager().SetLeverage(req.Symbol, req.Leverage, req.MarginMode); err != nil {
		logger.Error("设置杠杆失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"symbol":      req.Symbol,
		"leverage":    req.Leverage,
		"margin_mode": req.MarginMode,
	})
}

func (s *Server) handleManualSetTrailingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req setTrailingStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.StopDistance <= 0 {
		http.Error(w, "stop_distance must be greater than 0", http.StatusBadRequest)
		return
	}

	if err := mgr.PositionManager().SetTrailingStop(req.Symbol, req.StopDistance); err != nil {
		logger.Error("设置移动止损失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"symbol":        req.Symbol,
		"stop_distance": req.StopDistance,
	})
}

func (s *Server) handleCreateTimedOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req createTimedOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.Side != "buy" && req.Side != "sell" {
		http.Error(w, "side must be 'buy' or 'sell'", http.StatusBadRequest)
		return
	}

	if req.Size <= 0 {
		http.Error(w, "size must be greater than 0", http.StatusBadRequest)
		return
	}

	if req.ExecuteAt == "" {
		http.Error(w, "execute_at is required", http.StatusBadRequest)
		return
	}

	executeAt, err := time.Parse(time.RFC3339, req.ExecuteAt)
	if err != nil {
		http.Error(w, "invalid execute_at format, use RFC3339 (e.g., 2024-01-01T15:04:05Z)", http.StatusBadRequest)
		return
	}

	order, err := mgr.TimedOrderManager().CreateOrder(req.Symbol, types.OrderSide(req.Side), req.Size, executeAt)
	if err != nil {
		logger.Error("创建限时单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"order":  order,
	})
}

func (s *Server) handleCancelTimedOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/api/manual/timed-order/")
	if orderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	if err := mgr.TimedOrderManager().CancelOrder(orderID); err != nil {
		logger.Error("取消限时单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"order_id": orderID,
	})
}

func (s *Server) handleListTimedOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	statusStr := r.URL.Query().Get("status")
	var orders []*manualtrading.TimedOrder
	if statusStr != "" {
		orders = mgr.TimedOrderManager().ListOrders(manualtrading.TimedOrderStatus(statusStr))
	} else {
		orders = mgr.TimedOrderManager().ListOrders("")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"orders": orders,
	})
}

func (s *Server) handleCreateConditionalOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	var req createConditionalOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.Side != "buy" && req.Side != "sell" {
		http.Error(w, "side must be 'buy' or 'sell'", http.StatusBadRequest)
		return
	}

	if req.Size <= 0 {
		http.Error(w, "size must be greater than 0", http.StatusBadRequest)
		return
	}

	if req.OrderType != "market" && req.OrderType != "limit" {
		http.Error(w, "order_type must be 'market' or 'limit'", http.StatusBadRequest)
		return
	}

	if req.ConditionalType == "" {
		http.Error(w, "conditional_type is required", http.StatusBadRequest)
		return
	}

	if req.Condition == nil {
		http.Error(w, "condition is required", http.StatusBadRequest)
		return
	}

	order, err := mgr.ConditionalOrderManager().CreateOrder(
		req.Symbol,
		types.OrderSide(req.Side),
		req.Size,
		types.OrderType(req.OrderType),
		manualtrading.ConditionalOrderType(req.ConditionalType),
		req.Condition,
		req.Price,
	)
	if err != nil {
		logger.Error("创建条件单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"order":  order,
	})
}

func (s *Server) handleCancelConditionalOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/api/manual/conditional-order/")
	if orderID == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	if err := mgr.ConditionalOrderManager().CancelOrder(orderID); err != nil {
		logger.Error("取消条件单失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"order_id": orderID,
	})
}

func (s *Server) handleListConditionalOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	mgr := s.manualTradeMgr
	s.mutex.RUnlock()

	if mgr == nil {
		http.Error(w, "Manual trading not enabled", http.StatusServiceUnavailable)
		return
	}

	statusStr := r.URL.Query().Get("status")
	var orders []*manualtrading.ConditionalOrder
	if statusStr != "" {
		orders = mgr.ConditionalOrderManager().ListOrders(manualtrading.ConditionalOrderStatus(statusStr))
	} else {
		orders = mgr.ConditionalOrderManager().ListOrders("")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"orders": orders,
	})
}

func (s *Server) handleGetTicker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTC-USDT"
	}

	ticker, err := s.actions.GetTicker(symbol)
	if err != nil {
		logger.Error("获取行情失败", zap.String("symbol", symbol), zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, ticker)
}

func (s *Server) handleGetBars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTC-USDT"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	bars, err := s.actions.GetBars(symbol, interval, limit)
	if err != nil {
		logger.Error("获取K线失败", zap.String("symbol", symbol), zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bars)
}

func (s *Server) handleGetOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTC-USDT"
	}

	depthStr := r.URL.Query().Get("depth")
	depth := 20
	if depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}

	orderBook, err := s.actions.GetOrderBook(symbol, depth)
	if err != nil {
		logger.Error("获取订单簿失败", zap.String("symbol", symbol), zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, orderBook)
}

// 大模型分析处理函数
func (s *Server) handleLLMAnalyzeTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	analyzer := s.analyzer
	s.mutex.RUnlock()

	if analyzer == nil {
		http.Error(w, "LLM analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	var req analyzeTradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	tradeData := &llmanalysis.TradeDecisionData{
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSize: req.Size,
		EntryPrice:   req.Price,
	}

	result, err := analyzer.AnalyzeTrade(r.Context(), tradeData)
	if err != nil {
		logger.Error("交易分析失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":    result.Summary,
		"analysis":   result.Content,
		"risk_level": result.RiskLevel,
	})
}

func (s *Server) handleLLMAnalyzePositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	analyzer := s.analyzer
	s.mutex.RUnlock()

	if analyzer == nil {
		http.Error(w, "LLM analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	result, err := analyzer.AnalyzePosition(r.Context(), "positions", nil)
	if err != nil {
		logger.Error("持仓分析失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":    result.Summary,
		"analysis":   result.Content,
		"risk_level": result.RiskLevel,
	})
}

func (s *Server) handleLLMAnalyzeMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	analyzer := s.analyzer
	s.mutex.RUnlock()

	if analyzer == nil {
		http.Error(w, "LLM analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	var req analyzeMarketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		req.Symbol = "BTC-USDT"
	}

	result, err := analyzer.AnalyzeMarket(r.Context(), []string{req.Symbol})
	if err != nil {
		logger.Error("市场分析失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":    result.Summary,
		"analysis":   result.Content,
		"risk_level": result.RiskLevel,
	})
}

func (s *Server) handleLLMAnalyzeOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	analyzer := s.analyzer
	s.mutex.RUnlock()

	if analyzer == nil {
		http.Error(w, "LLM analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Orders       []map[string]interface{} `json:"orders"`
		TimeRange    string                   `json:"time_range"`
		AnalysisType string                   `json:"analysis_type"`
		Symbol       string                   `json:"symbol"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Orders) == 0 {
		http.Error(w, "orders is required", http.StatusBadRequest)
		return
	}

	data := &llmanalysis.OrderData{
		Orders:       req.Orders,
		TimeRange:    req.TimeRange,
		AnalysisType: req.AnalysisType,
		Symbol:       req.Symbol,
	}

	result, err := analyzer.AnalyzeOrders(r.Context(), data)
	if err != nil {
		logger.Error("订单分析失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":    result.Summary,
		"analysis":   result.Content,
		"risk_level": result.RiskLevel,
	})
}

func (s *Server) handleLLMHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	analyzer := s.analyzer
	s.mutex.RUnlock()

	if analyzer == nil {
		http.Error(w, "LLM analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	results, err := analyzer.ListAnalyses("", limit)
	if err != nil {
		logger.Error("获取分析历史失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"analyses": results,
	})
}

func (s *Server) handleGetNews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	dataService := s.dataService
	s.mutex.RUnlock()

	if dataService == nil {
		http.Error(w, "Data service not enabled", http.StatusServiceUnavailable)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	news, err := dataService.GetLatestNews(limit)
	if err != nil {
		logger.Error("获取新闻失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"news": news,
	})
}

func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	dataService := s.dataService
	s.mutex.RUnlock()

	if dataService == nil {
		http.Error(w, "Data service not enabled", http.StatusServiceUnavailable)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	events, err := dataService.GetUpcomingEvents(days)
	if err != nil {
		logger.Error("获取经济事件失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
	})
}

func (s *Server) handleCollectNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	dataService := s.dataService
	s.mutex.RUnlock()

	if dataService == nil {
		http.Error(w, "Data service not enabled", http.StatusServiceUnavailable)
		return
	}

	dataService.CollectNow()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "collecting",
	})
}

type sendAlertRequest struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Symbol  string `json:"symbol,omitempty"`
}

// 技术指标API请求结构
type calculateIndicatorRequest struct {
	Symbol     string            `json:"symbol"`
	Interval   string            `json:"interval"`
	Limit      int               `json:"limit,omitempty"`
	Indicators []IndicatorConfig `json:"indicators"`
}

type IndicatorConfig struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params,omitempty"`
}

func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	alertService := s.alertService
	s.mutex.RUnlock()

	if alertService == nil {
		http.Error(w, "Alert service not enabled", http.StatusServiceUnavailable)
		return
	}

	alertType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var alerts []*storage.AlertRecord
	var err error

	if alertType != "" {
		alerts, err = alertService.GetAlertsByType(alertType, limit)
	} else {
		alerts, err = alertService.GetRecentAlerts(limit)
	}

	if err != nil {
		logger.Error("获取提醒失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
	})
}

func (s *Server) handleSendAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	alertService := s.alertService
	s.mutex.RUnlock()

	if alertService == nil {
		http.Error(w, "Alert service not enabled", http.StatusServiceUnavailable)
		return
	}

	var req sendAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Message == "" {
		http.Error(w, "title and message are required", http.StatusBadRequest)
		return
	}

	alert := &alertservice.Alert{
		Type:    alertservice.AlertType(req.Type),
		Level:   alertservice.AlertLevel(req.Level),
		Title:   req.Title,
		Message: req.Message,
		Symbol:  req.Symbol,
	}

	if alert.Type == "" {
		alert.Type = alertservice.AlertTypeSystem
	}
	if alert.Level == "" {
		alert.Level = alertservice.AlertLevelInfo
	}

	if err := alertService.SendAlert(alert); err != nil {
		logger.Error("发送提醒失败", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "sent",
		"alert":  alert,
	})
}

// handleListIndicators 获取支持的技术指标列表
func (s *Server) handleListIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	// 返回支持的指标列表和参数说明
	indicators := []map[string]interface{}{
		{
			"name":        "MACD",
			"description": "移动平均收敛发散指标",
			"default_params": map[string]interface{}{
				"fast_period":   12,
				"slow_period":   26,
				"signal_period": 9,
			},
		},
		{
			"name":        "RSI",
			"description": "相对强弱指数",
			"default_params": map[string]interface{}{
				"period": 14,
			},
		},
		{
			"name":        "BOLLINGER",
			"description": "布林带指标",
			"default_params": map[string]interface{}{
				"period":  20,
				"std_dev": 2.0,
			},
		},
		{
			"name":        "ATR",
			"description": "平均真实波动范围",
			"default_params": map[string]interface{}{
				"period": 14,
			},
		},
		{
			"name":        "ADX",
			"description": "平均趋向指数",
			"default_params": map[string]interface{}{
				"period": 14,
			},
		},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"indicators": indicators,
	})
}

// handleCalculateIndicators 计算技术指标
func (s *Server) handleCalculateIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	if s.actions == nil || s.actions.GetBars == nil {
		http.Error(w, "Market data service not available", http.StatusServiceUnavailable)
		return
	}

	var req calculateIndicatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	if req.Interval == "" {
		req.Interval = "1m"
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 200 // 默认获取200根K线足够计算大多数指标
	}

	// 最少需要的K线数量
	minLimit := 100
	if limit < minLimit {
		limit = minLimit
	}

	// 获取K线数据
	bars, err := s.actions.GetBars(req.Symbol, req.Interval, limit)
	if err != nil {
		logger.Error("获取K线数据失败",
			zap.String("symbol", req.Symbol),
			zap.String("interval", req.Interval),
			zap.Error(err),
		)
		http.Error(w, fmt.Sprintf("获取K线数据失败: %v", err), http.StatusInternalServerError)
		return
	}

	if len(bars) < minLimit {
		http.Error(w, fmt.Sprintf("K线数据不足，需要至少%d根，当前只有%d根", minLimit, len(bars)), http.StatusBadRequest)
		return
	}

	// 创建指标集
	is := indicator.NewIndicatorSet()

	// 注册需要计算的指标
	for _, indConfig := range req.Indicators {
		var ind indicator.Indicator

		switch strings.ToUpper(indConfig.Name) {
		case "MACD":
			fastPeriod := 12
			slowPeriod := 26
			signalPeriod := 9

			if indConfig.Params != nil {
				if v, ok := indConfig.Params["fast_period"].(float64); ok && v > 0 {
					fastPeriod = int(v)
				}
				if v, ok := indConfig.Params["slow_period"].(float64); ok && v > 0 {
					slowPeriod = int(v)
				}
				if v, ok := indConfig.Params["signal_period"].(float64); ok && v > 0 {
					signalPeriod = int(v)
				}
			}

			ind = indicator.NewMACD(fastPeriod, slowPeriod, signalPeriod)

		case "RSI":
			period := 14
			if indConfig.Params != nil {
				if v, ok := indConfig.Params["period"].(float64); ok && v > 0 {
					period = int(v)
				}
			}
			ind = indicator.NewRSI(period)

		case "BOLLINGER", "BB":
			period := 20
			stdDev := 2.0
			if indConfig.Params != nil {
				if v, ok := indConfig.Params["period"].(float64); ok && v > 0 {
					period = int(v)
				}
				if v, ok := indConfig.Params["std_dev"].(float64); ok && v > 0 {
					stdDev = v
				}
			}
			ind = indicator.NewBollinger(period, stdDev)

		case "ATR":
			period := 14
			if indConfig.Params != nil {
				if v, ok := indConfig.Params["period"].(float64); ok && v > 0 {
					period = int(v)
				}
			}
			ind = indicator.NewATR(period)

		case "ADX":
			period := 14
			if indConfig.Params != nil {
				if v, ok := indConfig.Params["period"].(float64); ok && v > 0 {
					period = int(v)
				}
			}
			ind = indicator.NewADX(period)

		default:
			http.Error(w, fmt.Sprintf("不支持的指标类型: %s", indConfig.Name), http.StatusBadRequest)
			return
		}

		is.AddIndicator(indConfig.Name, ind)
	}

	// 计算指标
	results, err := is.CalculateAll(bars)
	if err != nil {
		logger.Error("计算指标失败", zap.Error(err))
		http.Error(w, fmt.Sprintf("计算指标失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 转换结果格式
	response := make(map[string]interface{})
	response["symbol"] = req.Symbol
	response["interval"] = req.Interval
	response["bar_count"] = len(bars)
	response["latest_bar_time"] = bars[len(bars)-1].Timestamp
	response["indicators"] = results

	writeJSON(w, http.StatusOK, response)
}

// handleGetHistoryData 获取历史K线数据
func (s *Server) handleGetHistoryData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTC-USDT"
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if s.actions != nil && s.actions.GetBars != nil {
		bars, err := s.actions.GetBars(symbol, interval, limit)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"symbol":    symbol,
				"interval":  interval,
				"bar_count": len(bars),
				"bars":      bars,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":    symbol,
		"interval":  interval,
		"bar_count": 0,
		"bars":      []interface{}{},
		"message":   "历史数据功能开发中",
	})
}

// handleGetTickData 获取历史行情数据
func (s *Server) handleGetTickData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTC-USDT"
	}

	if s.actions != nil && s.actions.GetTicker != nil {
		ticker, err := s.actions.GetTicker(symbol)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"symbol":  symbol,
				"ticker":  ticker,
				"message": "实时行情数据",
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"ticker":  nil,
		"message": "历史行情数据功能开发中",
	})
}

// 回测API相关结构
type backtestStartRequest struct {
	StrategyName   string                 `json:"strategy_name"`
	StrategyParams map[string]interface{} `json:"strategy_params,omitempty"`
	Symbol         string                 `json:"symbol"`
	Interval       string                 `json:"interval"`
	Limit          int                    `json:"limit,omitempty"`
	InitialBalance float64                `json:"initial_balance,omitempty"`
}

type backtestTask struct {
	ID         string
	Status     string
	Strategy   string
	Result     *backtest.Result
	Report     *backtest.Report
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Error      string
}

var (
	backtestTasks       = make(map[string]*backtestTask)
	backtestTasksMutex  sync.RWMutex
)

// cleanupExpiredBacktestTasks 清理已完成超过 1 小时的回测任务
func cleanupExpiredBacktestTasks() {
	now := time.Now()
	backtestTasksMutex.Lock()
	defer backtestTasksMutex.Unlock()
	for id, task := range backtestTasks {
		if task.Status == "completed" || task.Status == "failed" {
			if task.FinishedAt != nil && now.Sub(*task.FinishedAt) > time.Hour {
				delete(backtestTasks, id)
			}
		}
	}
}

// handleBacktestStrategies 获取可用的回测策略列表
func (s *Server) handleBacktestStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	strategies := []map[string]interface{}{
		{
			"name":        "TrendFollowing",
			"description": "趋势跟随策略",
			"default_params": map[string]interface{}{
				"ema_short_period":      12,
				"ema_long_period":       26,
				"adx_period":            14,
				"adx_threshold":         25.0,
				"stop_loss_percent":     0.05,
				"trailing_stop_percent": 0.03,
			},
		},
		{
			"name":        "MeanReversion",
			"description": "均值回归策略",
			"default_params": map[string]interface{}{
				"lookback_period": 20,
				"entry_threshold": 2.0,
				"exit_threshold":  0.5,
			},
		},
		{
			"name":        "VolatilityBreakout",
			"description": "波动率突破策略",
			"default_params": map[string]interface{}{
				"lookback_period": 20,
				"multiplier":      2.0,
			},
		},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"strategies": strategies,
	})
}

// handleBacktestStart 启动回测
func (s *Server) handleBacktestStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	var req backtestStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StrategyName == "" {
		http.Error(w, "strategy_name is required", http.StatusBadRequest)
		return
	}
	if req.Symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}
	if req.Interval == "" {
		req.Interval = "1m"
	}
	if req.InitialBalance <= 0 {
		req.InitialBalance = 10000
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 500
	}

	taskID := fmt.Sprintf("backtest_%d", time.Now().UnixNano())
	task := &backtestTask{
		ID:        taskID,
		Status:    "pending",
		Strategy:  req.StrategyName,
		CreatedAt: time.Now(),
	}

	backtestTasksMutex.Lock()
	backtestTasks[taskID] = task
	backtestTasksMutex.Unlock()

	go func() {
		startedAt := time.Now()
		task.Status = "running"
		task.StartedAt = &startedAt

		defer func() {
			finishedAt := time.Now()
			task.FinishedAt = &finishedAt
		}()

		if s.actions == nil || s.actions.GetBars == nil {
			task.Status = "failed"
			task.Error = "Market data service not available"
			return
		}

		bars, err := s.actions.GetBars(req.Symbol, req.Interval, limit)
		if err != nil {
			task.Status = "failed"
			task.Error = fmt.Sprintf("获取K线数据失败: %v", err)
			return
		}

		var strat backtest.Strategy
		switch req.StrategyName {
		case "TrendFollowing":
			trendStrat := strategy.NewTrendFollowingStrategy()
			if req.StrategyParams != nil {
				if err := trendStrat.Init(req.StrategyParams); err != nil {
					task.Status = "failed"
					task.Error = fmt.Sprintf("初始化策略失败: %v", err)
					return
				}
			}
			strat = backtest.NewStrategyAdapter(trendStrat)
		default:
			task.Status = "failed"
			task.Error = fmt.Sprintf("不支持的策略: %s", req.StrategyName)
			return
		}

		if req.StrategyParams != nil {
			if err := strat.Init(req.StrategyParams); err != nil {
				task.Status = "failed"
				task.Error = fmt.Sprintf("初始化策略失败: %v", err)
				return
			}
		}

		engine := backtest.NewEngine(strat, req.InitialBalance)
		if err := engine.AddData(req.Symbol, bars); err != nil {
			task.Status = "failed"
			task.Error = fmt.Sprintf("添加数据失败: %v", err)
			return
		}

		if err := engine.Run(); err != nil {
			task.Status = "failed"
			task.Error = fmt.Sprintf("回测执行失败: %v", err)
			return
		}

		task.Result = engine.GetResult()
		reportGen := backtest.NewReportGenerator(task.Result)
		task.Report = reportGen.Generate(req.StrategyName)
		task.Status = "completed"
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"status":  "started",
		"message": "回测已启动",
	})
}

// handleBacktestGetResults 获取回测结果
func (s *Server) handleBacktestGetResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	cleanupExpiredBacktestTasks()

	taskID := strings.TrimPrefix(r.URL.Path, "/api/backtest/results/")
	if taskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}

	backtestTasksMutex.RLock()
	task, exists := backtestTasks[taskID]
	backtestTasksMutex.RUnlock()

	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"task_id":    task.ID,
		"status":     task.Status,
		"strategy":   task.Strategy,
		"created_at": task.CreatedAt,
	}

	if task.StartedAt != nil {
		response["started_at"] = task.StartedAt
	}
	if task.FinishedAt != nil {
		response["finished_at"] = task.FinishedAt
	}
	if task.Error != "" {
		response["error"] = task.Error
	}
	if task.Result != nil {
		response["result"] = task.Result
	}

	writeJSON(w, http.StatusOK, response)
}

// handleBacktestGetReport 获取回测报告
func (s *Server) handleBacktestGetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/api/backtest/report/")
	if taskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	backtestTasksMutex.RLock()
	task, exists := backtestTasks[taskID]
	backtestTasksMutex.RUnlock()

	if !exists {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	if task.Status != "completed" {
		http.Error(w, "回测尚未完成", http.StatusBadRequest)
		return
	}

	if task.Report == nil {
		http.Error(w, "报告尚未生成", http.StatusInternalServerError)
		return
	}

	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(task.Report.ToString()))
		return
	}

	writeJSON(w, http.StatusOK, task.Report)
}

// SetMetrics 设置指标管理器
func (s *Server) SetMetrics(metrics *monitoring.Metrics) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.metrics = metrics
}

// SetStrategyEngine 设置策略引擎
func (s *Server) SetStrategyEngine(engine *strategy.Engine) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.strategyEngine = engine
}

// handleStrategyParams 处理策略参数请求
func (s *Server) handleStrategyParams(w http.ResponseWriter, r *http.Request) {
	strategyName := strings.TrimPrefix(r.URL.Path, "/api/strategy/params/")

	switch r.Method {
	case http.MethodGet:
		s.getStrategyParams(w, r, strategyName)
	case http.MethodPut, http.MethodPost:
		s.setStrategyParams(w, r, strategyName)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getStrategyParams 获取策略参数
func (s *Server) getStrategyParams(w http.ResponseWriter, r *http.Request, strategyName string) {
	if !s.requireReadAccess(w, r) {
		return
	}
	s.mutex.RLock()
	engine := s.strategyEngine
	s.mutex.RUnlock()

	if engine == nil {
		http.Error(w, "Strategy engine not initialized", http.StatusServiceUnavailable)
		return
	}

	if strategyName == "" {
		configs := engine.GetAllStrategyConfigs()
		result := make([]map[string]interface{}, 0, len(configs))
		for name, config := range configs {
			paramInfo := map[string]interface{}{
				"name":    name,
				"params":  config.Params,
				"weight":  config.Weight,
				"enabled": config.Enabled,
			}

			if strat := engine.GetStrategy(name); strat != nil {
				if schema, ok := strat.(strategy.StrategyWithSchema); ok {
					paramInfo["schema"] = schema.GetParamSchema()
				}
			}

			result = append(result, paramInfo)
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	if !engine.HasStrategy(strategyName) {
		http.Error(w, "Strategy not found", http.StatusNotFound)
		return
	}

	params := engine.GetStrategyParams(strategyName)
	config := engine.GetStrategyConfig(strategyName)

	result := map[string]interface{}{
		"name":    strategyName,
		"params":  params,
		"weight":  config.Weight,
		"enabled": config.Enabled,
	}

	if strat := engine.GetStrategy(strategyName); strat != nil {
		if schema, ok := strat.(strategy.StrategyWithSchema); ok {
			result["schema"] = schema.GetParamSchema()
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// setStrategyParams 设置策略参数
func (s *Server) setStrategyParams(w http.ResponseWriter, r *http.Request, strategyName string) {
	if strategyName == "" {
		http.Error(w, "Strategy name is required", http.StatusBadRequest)
		return
	}

	if err := s.requireMutationAccess(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !s.checkRateLimit(w, r) {
		return
	}

	s.mutex.RLock()
	engine := s.strategyEngine
	s.mutex.RUnlock()

	if engine == nil {
		http.Error(w, "Strategy engine not initialized", http.StatusServiceUnavailable)
		return
	}

	if !engine.HasStrategy(strategyName) {
		http.Error(w, "Strategy not found", http.StatusNotFound)
		return
	}

	var req struct {
		Params  map[string]interface{} `json:"params"`
		Weight  *float64               `json:"weight,omitempty"`
		Enabled *bool                  `json:"enabled,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Params != nil {
		if strat := engine.GetStrategy(strategyName); strat != nil {
			if schemaStrat, ok := strat.(strategy.StrategyWithSchema); ok {
				schema := schemaStrat.GetParamSchema()
				validator := strategy.NewParamValidator(schema)
				if err := validator.Validate(req.Params); err != nil {
					http.Error(w, fmt.Sprintf("Parameter validation failed: %v", err), http.StatusBadRequest)
					return
				}
				req.Params = validator.ApplyDefaults(req.Params)
			}
		}

		if err := engine.SetStrategyParams(strategyName, req.Params); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.Weight != nil {
		if err := engine.SetStrategyWeight(strategyName, *req.Weight); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.Enabled != nil {
		if *req.Enabled {
			engine.EnableStrategy(strategyName)
		} else {
			engine.DisableStrategy(strategyName)
		}
	}

	logger.Info("策略参数已更新",
		zap.String("strategy", strategyName),
	)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Strategy parameters updated",
	})
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.metrics == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"error": "metrics not initialized",
		})
		return
	}

	if s.metrics.GetSystemMetrics() != nil {
		s.metrics.GetSystemMetrics().Update()
	}

	writeJSON(w, http.StatusOK, s.metrics.GetAllMetrics())
}

func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireReadAccess(w, r) {
		return
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.metrics == nil {
		http.Error(w, "metrics not initialized", http.StatusServiceUnavailable)
		return
	}

	if s.metrics.GetSystemMetrics() != nil {
		s.metrics.GetSystemMetrics().Update()
	}

	prom := s.metrics.GetPrometheusMetrics()
	if prom == nil {
		http.Error(w, "prometheus metrics not initialized", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	output := prom.FormatPrometheus()
	if output != "" {
		w.Write([]byte(output))
	}
}
