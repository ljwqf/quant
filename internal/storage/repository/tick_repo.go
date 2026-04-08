package repository

import (
	"database/sql"
	"time"

	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/pkg/logger"
	"go.uber.org/zap"
)

// TickRepository 行情数据仓库接口
type TickRepository interface {
	Create(tick *storage.TickData) error
	CreateBatch(ticks []*storage.TickData) error
	List(symbol string, limit, offset int) ([]*storage.TickData, error)
	GetLatest(symbol string) (*storage.TickData, error)
	DeleteOld(before time.Time) (int64, error)
}

// tickRepository 行情数据仓库实现
type tickRepository struct {
	db *sql.DB
}

// NewTickRepository 创建行情数据仓库
func NewTickRepository(db *sql.DB) TickRepository {
	return &tickRepository{db: db}
}

// Create 保存行情数据
func (r *tickRepository) Create(tick *storage.TickData) error {
	query := `
		INSERT INTO tick_data (symbol, price, open_24h, high_24h, low_24h, volume_24h, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query,
		tick.Symbol,
		tick.Price,
		tick.Open24h,
		tick.High24h,
		tick.Low24h,
		tick.Volume24h,
		tick.Timestamp,
		time.Now(),
	)
	if err != nil {
		logger.Error("保存行情数据失败", zap.Error(err))
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Warn("获取插入ID失败", zap.Error(err))
	} else {
		tick.ID = id
	}

	return nil
}

// CreateBatch 批量保存行情数据
func (r *tickRepository) CreateBatch(ticks []*storage.TickData) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `
		INSERT INTO tick_data (symbol, price, open_24h, high_24h, low_24h, volume_24h, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, tick := range ticks {
		_, err := stmt.Exec(
			tick.Symbol,
			tick.Price,
			tick.Open24h,
			tick.High24h,
			tick.Low24h,
			tick.Volume24h,
			tick.Timestamp,
			time.Now(),
		)
		if err != nil {
			tx.Rollback()
			logger.Error("批量保存行情数据失败", zap.Error(err))
			return err
		}
	}

	return tx.Commit()
}

// List 查询行情数据
func (r *tickRepository) List(symbol string, limit, offset int) ([]*storage.TickData, error) {
	query := `
		SELECT id, symbol, price, open_24h, high_24h, low_24h, volume_24h, timestamp, created_at
		FROM tick_data
		WHERE symbol = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, symbol, limit, offset)
	if err != nil {
		logger.Error("查询行情数据失败", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var ticks []*storage.TickData
	for rows.Next() {
		tick := &storage.TickData{}
		err := rows.Scan(
			&tick.ID,
			&tick.Symbol,
			&tick.Price,
			&tick.Open24h,
			&tick.High24h,
			&tick.Low24h,
			&tick.Volume24h,
			&tick.Timestamp,
			&tick.CreatedAt,
		)
		if err != nil {
			logger.Error("扫描行情数据失败", zap.Error(err))
			continue
		}
		ticks = append(ticks, tick)
	}

	return ticks, nil
}

// GetLatest 获取最新行情
func (r *tickRepository) GetLatest(symbol string) (*storage.TickData, error) {
	query := `
		SELECT id, symbol, price, open_24h, high_24h, low_24h, volume_24h, timestamp, created_at
		FROM tick_data
		WHERE symbol = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	row := r.db.QueryRow(query, symbol)
	tick := &storage.TickData{}
	err := row.Scan(
		&tick.ID,
		&tick.Symbol,
		&tick.Price,
		&tick.Open24h,
		&tick.High24h,
		&tick.Low24h,
		&tick.Volume24h,
		&tick.Timestamp,
		&tick.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		logger.Error("获取最新行情失败", zap.Error(err))
		return nil, err
	}

	return tick, nil
}

// DeleteOld 删除旧数据
func (r *tickRepository) DeleteOld(before time.Time) (int64, error) {
	query := "DELETE FROM tick_data WHERE timestamp < ?"
	result, err := r.db.Exec(query, before)
	if err != nil {
		logger.Error("删除旧行情数据失败", zap.Error(err))
		return 0, err
	}

	count, _ := result.RowsAffected()
	return count, nil
}
