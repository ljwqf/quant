package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange/okx"
	"github.com/ljwqf/quant/internal/monitoring"
	"github.com/ljwqf/quant/internal/risk"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	if err := logger.Init(cfg.Basic.LogLevel, cfg.Basic.LogFile); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	logger.Info("启动OKX量化交易系统",
		zap.String("app_name", cfg.Basic.AppName),
		zap.String("env", cfg.Basic.Env),
		zap.String("log_level", cfg.Basic.LogLevel),
	)

	// 初始化OKX客户端
	exchangeClient := okx.NewClient(&cfg.Exchange.OKX)
	if exchangeClient == nil {
		logger.Fatal("初始化OKX客户端失败")
	}

	// 初始化风险管理
	riskManager := risk.NewManager(&cfg.Risk, exchangeClient)

	// 初始化监控系统
	monitor := monitoring.NewMonitor(&monitoring.Config{
		Enable:        cfg.Monitoring.Enable,
		CheckInterval: cfg.Monitoring.CheckInterval,
		AlertThreshold: monitoring.AlertThreshold{
			MaxDrawdown:   cfg.Monitoring.AlertThreshold.MaxDrawdown,
			MaxLoss:       cfg.Monitoring.AlertThreshold.MaxLoss,
			PositionLimit: cfg.Monitoring.AlertThreshold.PositionLimit,
			OrderTimeout:  cfg.Monitoring.AlertThreshold.OrderTimeout,
		},
		MetricsConfig: monitoring.MetricsConfig{
			Enable:         cfg.Monitoring.Metrics.Enable,
			PushGatewayURL: cfg.Monitoring.Metrics.PushGatewayURL,
		},
		AlertConfig: monitoring.AlertConfig{
			Enable:     cfg.Monitoring.Alert.Enable,
			Channels:   cfg.Monitoring.Alert.Channels,
			WebhookURL: cfg.Monitoring.Alert.WebhookURL,
		},
	}, exchangeClient, riskManager)

	// 启动监控系统
	if err := monitor.Start(); err != nil {
		logger.Error("启动监控系统失败", zap.Error(err))
	}

	// 启动OKX客户端
	if err := exchangeClient.Connect(); err != nil {
		logger.Fatal("启动OKX客户端失败", zap.Error(err))
	}

	// 示例：使用回测系统
	if cfg.Backtest.Enable {
		logger.Info("启动回测系统")
		// 这里可以添加回测逻辑
	}

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("收到中断信号，正在关闭系统...")

	// 停止监控系统
	monitor.Stop()

	// 停止OKX客户端
	exchangeClient.Disconnect()

	logger.Info("系统已关闭")
}
