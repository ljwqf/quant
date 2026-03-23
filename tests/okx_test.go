package tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange/okx"
	"github.com/ljwqf/quant/pkg/types"
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
		fmt.Println("✅ 连接成功")
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

		fmt.Printf("✅ 账户信息: 总资产 %.2f USDT\n", account.TotalEquity)
		for currency, balance := range account.Balance {
			if balance.Total > 0 {
				fmt.Printf("  - %s: %.2f\n", currency, balance.Total)
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

		fmt.Printf("✅ 行情数据: %s 价格: %.2f USDT\n", ticker.Symbol, ticker.Price)
		fmt.Printf("  24h 开盘: %.2f, 最高: %.2f, 最低: %.2f\n", ticker.Open24h, ticker.High24h, ticker.Low24h)
		fmt.Printf("  24h 成交量: %.2f\n", ticker.Volume24h)
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

		fmt.Printf("✅ K线数据: %d 条\n", len(bars))
		for i, bar := range bars {
			if i < 3 { // 只显示前3条
				fmt.Printf("  %s: O=%.2f, H=%.2f, L=%.2f, C=%.2f, V=%.2f\n",
					bar.Timestamp.Format("15:04"), bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
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

		fmt.Printf("✅ 订单簿数据: %s\n", orderBook.Symbol)
		fmt.Println("  卖单:")
		for i, ask := range orderBook.Asks {
			if i < 3 {
				fmt.Printf("    %.2f: %.2f\n", ask.Price, ask.Size)
			}
		}
		fmt.Println("  买单:")
		for i, bid := range orderBook.Bids {
			if i < 3 {
				fmt.Printf("    %.2f: %.2f\n", bid.Price, bid.Size)
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
		fmt.Println("✅ 开始订阅行情数据...")
		if err := client.SubscribeTicker("BTC-USDT-SWAP", func(tick *types.Tick) {
			fmt.Printf("  行情更新: %s 价格: %.2f USDT\n", tick.Symbol, tick.Price)
		}); err != nil {
			t.Fatalf("订阅行情失败: %v", err)
		}

		// 订阅K线
		if err := client.SubscribeBar("BTC-USDT-SWAP", "1m", func(bar *types.Bar) {
			fmt.Printf("  K线更新: %s %s O=%.2f, C=%.2f\n", bar.Symbol, bar.Interval, bar.Open, bar.Close)
		}); err != nil {
			t.Fatalf("订阅K线失败: %v", err)
		}

		// 订阅订单簿
		if err := client.SubscribeOrderBook("BTC-USDT-SWAP", func(orderBook *types.OrderBook) {
			if len(orderBook.Asks) > 0 && len(orderBook.Bids) > 0 {
				bestAsk := orderBook.Asks[0].Price
				bestBid := orderBook.Bids[0].Price
				fmt.Printf("  订单簿更新: %s 买一: %.2f 卖一: %.2f\n", orderBook.Symbol, bestBid, bestAsk)
			}
		}); err != nil {
			t.Fatalf("订阅订单簿失败: %v", err)
		}

		// 等待 30 秒接收数据
		time.Sleep(30 * time.Second)
	})
}
