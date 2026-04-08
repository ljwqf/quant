package dataservice

import (
	"sync"
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/data/cryptoquant"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/storage/repository"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// DataService 数据采集服务
type DataService struct {
	cfg             *config.Config
	db              *storage.Database
	cryptoquant     *cryptoquant.Client
	newsRepo        repository.NewsEventRepository
	economicRepo    repository.EconomicEventRepository
	sourceManager   *SourceManager
	dataQueue       DataQueue
	running         bool
	stopCh          chan struct{}
	wg              sync.WaitGroup
	mutex           sync.RWMutex
}

// NewDataService 创建数据采集服务
func NewDataService(cfg *config.Config, db *storage.Database) *DataService {
	var cqClient *cryptoquant.Client
	if cfg.DataService.CryptoQuantEnable && cfg.DataService.CryptoQuantAPIKey != "" {
		cqClient = cryptoquant.NewClient(cfg.DataService.CryptoQuantAPIKey)
		logger.Info("CryptoQuant 客户端初始化成功")
	}

	service := &DataService{
		cfg:           cfg,
		db:            db,
		cryptoquant:   cqClient,
		newsRepo:      repository.NewNewsEventRepository(db.DB()),
		economicRepo:  repository.NewEconomicEventRepository(db.DB()),
		sourceManager: NewSourceManager(),
		dataQueue:     NewMemoryQueue(1000),
		stopCh:        make(chan struct{}),
	}

	service.initDefaultSources()
	return service
}

// initDefaultSources 初始化默认数据源
func (s *DataService) initDefaultSources() {
	if s.cfg.Exchange.OKX.APIKey != "" {
		okxSource := NewOKXSource()
		config := map[string]interface{}{
			"api_key":    s.cfg.Exchange.OKX.APIKey,
			"secret_key": s.cfg.Exchange.OKX.SecretKey,
			"passphrase": s.cfg.Exchange.OKX.Passphrase,
			"simulated":  s.cfg.Exchange.OKX.Simulated,
		}
		if err := okxSource.Initialize(config); err != nil {
			logger.Warn("初始化OKX数据源失败", zap.Error(err))
		} else {
			if err := s.sourceManager.RegisterSource(okxSource); err != nil {
				logger.Warn("注册OKX数据源失败", zap.Error(err))
			}
		}
	} else {
		logger.Info("OKX配置未提供，跳过OKX数据源初始化")
	}
}

// SourceManager 获取数据源管理器
func (s *DataService) SourceManager() *SourceManager {
	return s.sourceManager
}

// DataQueue 获取数据队列
func (s *DataService) DataQueue() DataQueue {
	return s.dataQueue
}

// Start 启动数据采集服务
func (s *DataService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return nil
	}

	s.running = true
	s.stopCh = make(chan struct{})

	logger.Info("数据采集服务启动")

	s.wg.Add(1)
	go s.collectDataLoop()

	return nil
}

// Stop 停止数据采集服务
func (s *DataService) Stop() {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mutex.Unlock()

	s.wg.Wait()
	logger.Info("数据采集服务已停止")
}

// IsRunning 检查服务是否运行
func (s *DataService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// collectDataLoop 数据采集循环
func (s *DataService) collectDataLoop() {
	defer s.wg.Done()

	interval := s.cfg.DataService.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.collectAllData()
		}
	}
}

// collectAllData 采集所有数据
func (s *DataService) collectAllData() {
	logger.Debug("开始采集数据")

	var wg sync.WaitGroup

	if s.cfg.DataService.NewsEnable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.collectNewsData()
		}()
	}

	if s.cfg.DataService.EconomicEnable {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.collectEconomicData()
		}()
	}

	if s.cfg.DataService.CryptoQuantEnable && s.cryptoquant != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.collectOnChainData()
		}()
	}

	wg.Wait()
	logger.Debug("数据采集完成")
}

// collectNewsData 采集新闻数据
func (s *DataService) collectNewsData() {
	logger.Debug("采集新闻数据")
	newsEvents := s.getMockNewsData()

	for _, event := range newsEvents {
		if err := s.newsRepo.Create(event); err != nil {
			logger.Warn("保存新闻事件失败", zap.Error(err))
		}
	}
}

// collectEconomicData 采集经济事件数据
func (s *DataService) collectEconomicData() {
	logger.Debug("采集经济事件数据")
	economicEvents := s.getMockEconomicData()

	for _, event := range economicEvents {
		if err := s.economicRepo.Create(event); err != nil {
			logger.Warn("保存经济事件失败", zap.Error(err))
		}
	}
}

// collectOnChainData 采集链上数据
func (s *DataService) collectOnChainData() {
	logger.Debug("采集链上数据")

	assets := []string{"BTC", "ETH"}
	for _, asset := range assets {
		netFlow, sopr, mvrv, err := s.cryptoquant.GetOnChainData(asset)
		if err != nil {
			logger.Warn("获取链上数据失败", zap.String("asset", asset), zap.Error(err))
			continue
		}

		logger.Debug("链上数据",
			zap.String("asset", asset),
			zap.Float64("net_flow", netFlow),
			zap.Float64("sopr", sopr),
			zap.Float64("mvrv", mvrv))
	}
}

// getMockNewsData 获取模拟新闻数据
func (s *DataService) getMockNewsData() []*storage.NewsEvent {
	return []*storage.NewsEvent{
		{
			Title:          "比特币价格突破新高",
			Source:         "CoinDesk",
			URL:            "https://example.com/news/bitcoin-high",
			PublishedAt:    time.Now(),
			Importance:     3,
			RelatedSymbols: "BTC-USDT",
			CreatedAt:      time.Now(),
		},
		{
			Title:          "以太坊升级计划公布",
			Source:         "The Block",
			URL:            "https://example.com/news/eth-upgrade",
			PublishedAt:    time.Now().Add(-1 * time.Hour),
			Importance:     2,
			RelatedSymbols: "ETH-USDT",
			CreatedAt:      time.Now(),
		},
	}
}

// getMockEconomicData 获取模拟经济数据
func (s *DataService) getMockEconomicData() []*storage.EconomicEvent {
	return []*storage.EconomicEvent{
		{
			Title:      "美联储利率决议",
			Country:    "US",
			EventTime:  time.Now().Add(24 * time.Hour),
			Importance: 3,
			Actual:     0.0,
			Forecast:   0.0,
			Previous:   0.0,
			CreatedAt:  time.Now(),
		},
		{
			Title:      "美国非农就业数据",
			Country:    "US",
			EventTime:  time.Now().Add(48 * time.Hour),
			Importance: 3,
			Actual:     0.0,
			Forecast:   0.0,
			Previous:   0.0,
			CreatedAt:  time.Now(),
		},
	}
}

// GetLatestNews 获取最新新闻
func (s *DataService) GetLatestNews(limit int) ([]*storage.NewsEvent, error) {
	return s.newsRepo.List(limit, 0)
}

// GetUpcomingEvents 获取即将到来的经济事件
func (s *DataService) GetUpcomingEvents(days int) ([]*storage.EconomicEvent, error) {
	return s.economicRepo.ListUpcoming(days)
}

// CollectNow 立即采集一次数据
func (s *DataService) CollectNow() {
	go s.collectAllData()
}
