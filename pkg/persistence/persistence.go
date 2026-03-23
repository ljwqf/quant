package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// Storage 存储接口
type Storage interface {
	Save(key string, data interface{}) error
	Load(key string, data interface{}) error
	LoadLatest(key string, data interface{}) error
	Delete(key string) error
	Exists(key string) bool
	ListKeys(pattern string) ([]string, error)
}

// FileStorage 文件系统存储
type FileStorage struct {
	dir string
}

// NewFileStorage 创建文件存储实例
func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &FileStorage{
		dir: dir,
	}, nil
}

// getFilePath 获取文件路径（带时间戳）
func (fs *FileStorage) getFilePath(key string) string {
	return filepath.Join(fs.dir, key+"_"+time.Now().Format("20060102_150405")+".json")
}

// getSimpleFilePath 获取简单文件路径（不带时间戳）
func (fs *FileStorage) getSimpleFilePath(key string) string {
	return filepath.Join(fs.dir, key+".json")
}

// Save 保存数据（带时间戳）
func (fs *FileStorage) Save(key string, data interface{}) error {
	filePath := fs.getFilePath(key)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return err
	}

	logger.Info("数据保存成功",
		zap.String("key", key),
		zap.String("file", filePath),
	)

	return nil
}

// SaveSimple 保存数据（不带时间戳，会覆盖）
func (fs *FileStorage) SaveSimple(key string, data interface{}) error {
	filePath := fs.getSimpleFilePath(key)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return err
	}

	logger.Info("数据保存成功",
		zap.String("key", key),
		zap.String("file", filePath),
	)

	return nil
}

// Load 加载数据（使用当前日期的文件）
func (fs *FileStorage) Load(key string, data interface{}) error {
	filePath := fs.getSimpleFilePath(key)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return err
	}

	logger.Info("数据加载成功",
		zap.String("key", key),
		zap.String("file", filePath),
	)

	return nil
}

// LoadLatest 加载最新的数据文件
func (fs *FileStorage) LoadLatest(key string, data interface{}) error {
	files, err := filepath.Glob(filepath.Join(fs.dir, key+"_*.json"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return os.ErrNotExist
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	filePath := files[0]

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return err
	}

	logger.Info("最新数据加载成功",
		zap.String("key", key),
		zap.String("file", filePath),
	)

	return nil
}

// Delete 删除数据
func (fs *FileStorage) Delete(key string) error {
	filePath := fs.getSimpleFilePath(key)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return os.ErrNotExist
	}

	if err := os.Remove(filePath); err != nil {
		return err
	}

	logger.Info("数据删除成功",
		zap.String("key", key),
		zap.String("file", filePath),
	)

	return nil
}

// Exists 检查数据是否存在
func (fs *FileStorage) Exists(key string) bool {
	filePath := fs.getSimpleFilePath(key)
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// ListKeys 列出匹配的键
func (fs *FileStorage) ListKeys(pattern string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(fs.dir, pattern))
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(files))
	for _, file := range files {
		keys = append(keys, filepath.Base(file))
	}

	return keys, nil
}

// TransactionRecord 交易记录
type TransactionRecord struct {
	ID           string    `json:"id"`
	Strategy     string    `json:"strategy"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	EntryPrice   float64   `json:"entry_price"`
	ExitPrice    float64   `json:"exit_price"`
	Quantity     float64   `json:"quantity"`
	PNL          float64   `json:"pnl"`
	PNLPercent   float64   `json:"pnl_percent"`
	EntryTime    time.Time `json:"entry_time"`
	ExitTime     time.Time `json:"exit_time"`
	HoldingTime  string    `json:"holding_time"`
	Status       string    `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// StrategyState 策略状态
type StrategyState struct {
	Name        string                 `json:"name"`
	Params      map[string]interface{} `json:"params"`
	Metrics     map[string]interface{} `json:"metrics"`
	Position    *PositionState         `json:"position"`
	LastUpdate  time.Time              `json:"last_update"`
}

// PositionState 持仓状态
type PositionState struct {
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	EntryPrice float64   `json:"entry_price"`
	Size       float64   `json:"size"`
	OpenTime   time.Time `json:"open_time"`
	TakeProfit float64   `json:"take_profit"`
	StopLoss   float64   `json:"stop_loss"`
}

// SaveTransaction 保存交易记录
func (fs *FileStorage) SaveTransaction(record *TransactionRecord) error {
	return fs.Save("transaction_"+record.ID, record)
}

// SaveStrategyState 保存策略状态
func (fs *FileStorage) SaveStrategyState(state *StrategyState) error {
	return fs.Save("strategy_"+state.Name, state)
}

// LoadStrategyState 加载策略状态
func (fs *FileStorage) LoadStrategyState(name string, state *StrategyState) error {
	return fs.Load("strategy_"+name, state)
}

// DeltaNeutralFundingState DeltaNeutralFunding策略状态
type DeltaNeutralFundingState struct {
	Name           string                 `json:"name"`
	Params         map[string]interface{} `json:"params"`
	State          int                    `json:"state"`
	SpotSymbol     string                 `json:"spot_symbol"`
	PerpSymbol     string                 `json:"perp_symbol"`
	SpotPosition   *DeltaPositionState    `json:"spot_position"`
	PerpPosition   *DeltaPositionState    `json:"perp_position"`
	SpotPrice      float64                `json:"spot_price"`
	PerpPrice      float64                `json:"perp_price"`
	DailyLoss      float64                `json:"daily_loss"`
	DailyLossReset time.Time              `json:"daily_loss_reset"`
	TotalPnL       float64                `json:"total_pnl"`
	FundingIncome  float64                `json:"funding_income"`
	TradeCount     int                    `json:"trade_count"`
	LastUpdate     time.Time              `json:"last_update"`
}

// DeltaPositionState Delta持仓状态
type DeltaPositionState struct {
	Symbol     string    `json:"symbol"`
	Side       int       `json:"side"`
	Size       float64   `json:"size"`
	EntryPrice float64   `json:"entry_price"`
	MarkPrice  float64   `json:"mark_price"`
	Value      float64   `json:"value"`
	Timestamp  time.Time `json:"timestamp"`
}

// SaveDeltaNeutralFundingState 保存DeltaNeutralFunding策略状态
func (fs *FileStorage) SaveDeltaNeutralFundingState(state *DeltaNeutralFundingState) error {
	return fs.SaveSimple("delta_neutral_funding_"+state.Name, state)
}

// LoadDeltaNeutralFundingState 加载DeltaNeutralFunding策略状态
func (fs *FileStorage) LoadDeltaNeutralFundingState(name string, state *DeltaNeutralFundingState) error {
	return fs.Load("delta_neutral_funding_"+name, state)
}

// LoadLatestDeltaNeutralFundingState 加载最新的DeltaNeutralFunding策略状态
func (fs *FileStorage) LoadLatestDeltaNeutralFundingState(name string, state *DeltaNeutralFundingState) error {
	return fs.LoadLatest("delta_neutral_funding_"+name, state)
}
