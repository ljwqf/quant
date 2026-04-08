package repository

import (
	"database/sql"
	"time"

	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// KlineRepository K线数据仓库接口
type KlineRepository interface {
	Create(kline *storage.KlineData) error
	CreateBatch(klines []*storage.KlineData) error
	List(symbol, interval string, limit, offset int) ([]*storage.KlineData, error)
	ListByTimeRange(symbol, interval string, startTime, endTime time.Time) ([]*storage.KlineData, error)
	DeleteOld(before time.Time) (int64, error)
}

// klineRepository K线数据仓库实现
type klineRepository struct {
	db *sql.DB
}

// NewKlineRepository 创建K线数据仓库
func NewKlineRepository(db *sql.DB) KlineRepository {
	return &klineRepository{db: db}
}

// Create 保存K线数据
func (r *klineRepository) Create(kline *storage.KlineData) error {
	query := `
		INSERT INTO kline_data (symbol, interval, open, high, low, close, volume, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query,
		kline.Symbol,
		kline.Interval,
		kline.Open,
		kline.High,
		kline.Low,
		kline.Close,
		kline.Volume,
		kline.Timestamp,
		time.Now(),
	)
	if err != nil {
		logger.Error("保存K线数据失败", zap.Error(err))
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Warn("获取插入ID失败", zap.Error(err))
	} else {
		kline.ID = id
	}

	return nil
}

// CreateBatch 批量保存K线数据
func (r *klineRepository) CreateBatch(klines []*storage.KlineData) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO kline_data (symbol, interval, open, high, low, close, volume, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, kline := range klines {
		_, err := stmt.Exec(
			kline.Symbol,
			kline.Interval,
			kline.Open,
			kline.High,
			kline.Low,
			kline.Close,
			kline.Volume,
			kline.Timestamp,
			time.Now(),
		)
		if err != nil {
			tx.Rollback()
			logger.Error("批量保存K线数据失败", zap.Error(err))
			return err
		}
	}

	return tx.Commit()
}

// List 查询K线数据
func (r *klineRepository) List(symbol, interval string, limit, offset int) ([]*storage.KlineData, error) {
	query := `
		SELECT id, symbol, interval, open, high, low, close, volume, timestamp, created_at
		FROM kline_data
		WHERE symbol = ? AND interval = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, symbol, interval, limit, offset)
	if err != nil {
		logger.Error("查询K线数据失败", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var klines []*storage.KlineData
	for rows.Next() {
		kline := &storage.KlineData{}
		err := rows.Scan(
			&kline.ID,
			&kline.Symbol,
			&kline.Interval,
			&kline.Open,
			&kline.High,
			&kline.Low,
			&kline.Close,
			&kline.Volume,
			&kline.Timestamp,
			&kline.CreatedAt,
		)
		if err != nil {
			logger.Error("扫描K线数据失败", zap.Error(err))
			continue
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

// ListByTimeRange 按时间范围查询K线数据
func (r *klineRepository) ListByTimeRange(symbol, interval string, startTime, endTime time.Time) ([]*storage.KlineData, error) {
	query := `
		SELECT id, symbol, interval, open, high, low, close, volume, timestamp, created_at
		FROM kline_data
		WHERE symbol = ? AND interval = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`

	rows, err := r.db.Query(query, symbol, interval, startTime, endTime)
	if err != nil {
		logger.Error("查询K线数据失败", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var klines []*storage.KlineData
	for rows.Next() {
		kline := &storage.KlineData{}
		err := rows.Scan(
			&kline.ID,
			&kline.Symbol,
			&kline.Interval,
			&kline.Open,
			&kline.High,
			&kline.Low,
			&kline.Close,
			&kline.Volume,
			&kline.Timestamp,
			&kline.CreatedAt,
		)
		if err != nil {
			logger.Error("扫描K线数据失败", zap.Error(err))
			continue
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

// DeleteOld 删除旧数据
func (r *klineRepository) DeleteOld(before time.Time) (int64, error) {
	query := "DELETE FROM kline_data WHERE timestamp < ?"
	result, err := r.db.Exec(query, before)
	if err != nil {
		logger.Error("删除旧K线数据失败", zap.Error(err))
		return 0, err
	}

	count, _ := result.RowsAffected()
	return count, nil
}
