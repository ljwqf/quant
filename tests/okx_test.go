package tests

import (
	"os"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange/okx"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

// TestOKXClient 测试 OKX 客户端（集成测试，需要配置文件和API密钥）
func TestOKXClient(t *testing.T) {
	// 检查配置文件是否存在
	configPath := "configs/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("跳过集成测试: 配置文件不存在，请复制 config.yaml.example 为 config.yaml 并配置API密钥")
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 检查API密钥是否配置
	if cfg.Exchange.OKX.APIKey == "" || cfg.Exchange.OKX.APIKey == "${OKX_API_KEY}" {
		t.Skip("跳过集成测试: 请在配置文件或环境变量中设置OKX API密钥")
	}

	// 初始化日志
	if err := logger.Init("info", "logs/test-okx.log"); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 创建 OKX 客户端
	client := okx.NewClient(&cfg.Exchange.OKX)

	// 测试连接
	t.Run("Connect", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		if !client.IsConnected() {
			t.Fatal("连接状态检查失败")
		}
		logger.Info("✅ 连接成功")
	})

	// 测试获取账户信息
	t.Run("GetAccount", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		account, err := client.GetAccount()
		if err != nil {
			t.Fatalf("获取账户信息失败: %v", err)
		}

		logger.Info("✅ 账户信息",
			zap.Float64("total_equity", account.TotalEquity),
		)
		for currency, balance := range account.Balance {
			if balance.Total > 0 {
				logger.Info("账户余额",
					zap.String("currency", currency),
					zap.Float64("total", balance.Total),
				)
			}
		}
	})

	// 测试获取行情
	t.Run("GetTicker", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		ticker, err := client.GetTicker("BTC-USDT-SWAP")
		if err != nil {
			t.Fatalf("获取行情失败: %v", err)
		}

		logger.Info("✅ 行情数据",
			zap.String("symbol", ticker.Symbol),
			zap.Float64("price", ticker.Price),
			zap.Float64("open_24h", ticker.Open24h),
			zap.Float64("high_24h", ticker.High24h),
			zap.Float64("low_24h", ticker.Low24h),
			zap.Float64("volume_24h", ticker.Volume24h),
		)
	})

	// 测试获取K线
	t.Run("GetBars", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		bars, err := client.GetBars("BTC-USDT-SWAP", "1m", 10)
		if err != nil {
			t.Fatalf("获取K线失败: %v", err)
		}

		logger.Info("✅ K线数据", zap.Int("count", len(bars)))
		for i, bar := range bars {
			if i < 3 { // 只显示前3条
				logger.Info("K线",
					zap.String("time", bar.Timestamp.Format("15:04")),
					zap.Float64("open", bar.Open),
					zap.Float64("high", bar.High),
					zap.Float64("low", bar.Low),
					zap.Float64("close", bar.Close),
					zap.Float64("volume", bar.Volume),
				)
			}
		}
	})

	// 测试获取订单簿
	t.Run("GetOrderBook", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		orderBook, err := client.GetOrderBook("BTC-USDT-SWAP", 5)
		if err != nil {
			t.Fatalf("获取订单簿失败: %v", err)
		}

		logger.Info("✅ 订单簿数据", zap.String("symbol", orderBook.Symbol))
		logger.Info("卖单")
		for i, ask := range orderBook.Asks {
			if i < 3 {
				logger.Info("卖单",
					zap.Float64("price", ask.Price),
					zap.Float64("size", ask.Size),
				)
			}
		}
		logger.Info("买单")
		for i, bid := range orderBook.Bids {
			if i < 3 {
				logger.Info("买单",
					zap.Float64("price", bid.Price),
					zap.Float64("size", bid.Size),
				)
			}
		}
	})

	// 测试 WebSocket 订阅
	t.Run("WebSocketSubscribe", func(t *testing.T) {
		if err := client.Connect(); err != nil {
			t.Fatalf("连接失败: %v", err)
		}
		defer client.Disconnect()

		// 订阅行情
		logger.Info("✅ 开始订阅行情数据...")
		if err := client.SubscribeTicker("BTC-USDT-SWAP", func(tick *types.Tick) {
			logger.Info("行情更新",
				zap.String("symbol", tick.Symbol),
				zap.Float64("price", tick.Price),
			)
		}); err != nil {
			t.Fatalf("订阅行情失败: %v", err)
		}

		// 订阅K线
		if err := client.SubscribeBar("BTC-USDT-SWAP", "1m", func(bar *types.Bar) {
			logger.Info("K线更新",
				zap.String("symbol", bar.Symbol),
				zap.String("interval", bar.Interval),
				zap.Float64("open", bar.Open),
				zap.Float64("close", bar.Close),
			)
		}); err != nil {
			t.Fatalf("订阅K线失败: %v", err)
		}

		// 订阅订单簿
		if err := client.SubscribeOrderBook("BTC-USDT-SWAP", func(orderBook *types.OrderBook) {
			if len(orderBook.Asks) > 0 && len(orderBook.Bids) > 0 {
				bestAsk := orderBook.Asks[0].Price
				bestBid := orderBook.Bids[0].Price
				logger.Info("订单簿更新",
					zap.String("symbol", orderBook.Symbol),
					zap.Float64("best_bid", bestBid),
					zap.Float64("best_ask", bestAsk),
				)
			}
		}); err != nil {
			t.Fatalf("订阅订单簿失败: %v", err)
		}

		// 等待 30 秒接收数据
		time.Sleep(30 * time.Second)
	})
}