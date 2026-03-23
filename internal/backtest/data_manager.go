package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/ljwqf/quant/pkg/types"
)

// DataManager 数据管理器
type DataManager struct {
	data map[string][]*types.Bar
}

// NewDataManager 创建数据管理器
func NewDataManager() *DataManager {
	return &DataManager{
		data: make(map[string][]*types.Bar),
	}
}

// AddData 添加数据
func (dm *DataManager) AddData(symbol string, bars []*types.Bar) error {
	if _, ok := dm.data[symbol]; !ok {
		dm.data[symbol] = make([]*types.Bar, 0)
	}
	dm.data[symbol] = append(dm.data[symbol], bars...)
	return nil
}

// LoadFromFile 从文件加载数据
func (dm *DataManager) LoadFromFile(filePath string, symbol string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	bars := make([]*types.Bar, 0, len(records)-1)
	for i, record := range records {
		if i == 0 {
			continue // 跳过表头
		}

		if len(record) < 7 {
			continue
		}

		timestamp, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			continue
		}

		open, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			continue
		}

		high, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			continue
		}

		low, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			continue
		}

		close, err := strconv.ParseFloat(record[4], 64)
		if err != nil {
			continue
		}

		volume, err := strconv.ParseFloat(record[5], 64)
		if err != nil {
			continue
		}

		interval := record[6]

		bar := &types.Bar{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Timestamp: time.Unix(timestamp/1000, 0),
			Interval:  interval,
		}

		bars = append(bars, bar)
	}

	return dm.AddData(symbol, bars)
}

// GetData 获取指定符号的数据
func (dm *DataManager) GetData(symbol string) ([]*types.Bar, error) {
	bars, ok := dm.data[symbol]
	if !ok {
		return nil, fmt.Errorf("符号 %s 没有数据", symbol)
	}
	return bars, nil
}

// GetSortedData 获取按时间排序的所有数据
func (dm *DataManager) GetSortedData() ([]*types.Bar, error) {
	allBars := make([]*types.Bar, 0)

	for _, bars := range dm.data {
		allBars = append(allBars, bars...)
	}

	// 按时间排序
	sort.Slice(allBars, func(i, j int) bool {
		return allBars[i].Timestamp.Before(allBars[j].Timestamp)
	})

	return allBars, nil
}

// GetSymbols 获取所有符号
func (dm *DataManager) GetSymbols() []string {
	symbols := make([]string, 0, len(dm.data))
	for symbol := range dm.data {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetDataRange 获取指定时间范围的数据
func (dm *DataManager) GetDataRange(symbol string, startTime, endTime time.Time) ([]*types.Bar, error) {
	bars, err := dm.GetData(symbol)
	if err != nil {
		return nil, err
	}

	filtered := make([]*types.Bar, 0)
	for _, bar := range bars {
		if (bar.Timestamp.Equal(startTime) || bar.Timestamp.After(startTime)) &&
			(bar.Timestamp.Equal(endTime) || bar.Timestamp.Before(endTime)) {
			filtered = append(filtered, bar)
		}
	}

	return filtered, nil
}
