package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// AlertRecordRepository 提醒记录数据访问接口
type AlertRecordRepository interface {
	Create(record *storage.AlertRecord) error
	GetByID(id int64) (*storage.AlertRecord, error)
	List(limit, offset int) ([]*storage.AlertRecord, error)
	ListUnread(limit, offset int) ([]*storage.AlertRecord, error)
	ListRecent(limit int) ([]*storage.AlertRecord, error)
	ListByType(alertType string, limit, offset int) ([]*storage.AlertRecord, error)
	MarkAsRead(id int64) error
}

// alertRecordRepository 提醒记录数据访问实现
type alertRecordRepository struct {
	db *sql.DB
}

// NewAlertRecordRepository 创建提醒记录数据访问
func NewAlertRecordRepository(db *sql.DB) AlertRecordRepository {
	return &alertRecordRepository{db: db}
}

// Create 创建提醒记录
func (r *alertRecordRepository) Create(record *storage.AlertRecord) error {
	query := `
		INSERT INTO alert_records (
			alert_type, level, title, message, symbol, metadata, channels, read
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		record.AlertType, record.Level, record.Title, record.Message,
		record.Symbol, record.Metadata, record.Channels, record.Read,
	)
	if err != nil {
		return fmt.Errorf("创建提醒记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}

	record.ID = id
	return nil
}

// GetByID 根据ID获取提醒记录
func (r *alertRecordRepository) GetByID(id int64) (*storage.AlertRecord, error) {
	query := `
		SELECT id, alert_type, level, title, message, symbol, metadata, channels, read, created_at
		FROM alert_records WHERE id = ?
	`

	record := &storage.AlertRecord{}
	err := r.db.QueryRow(query, id).Scan(
		&record.ID, &record.AlertType, &record.Level, &record.Title, &record.Message,
		&record.Symbol, &record.Metadata, &record.Channels, &record.Read, &record.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询提醒记录失败: %w", err)
	}

	return record, nil
}

// List 列出提醒记录
func (r *alertRecordRepository) List(limit, offset int) ([]*storage.AlertRecord, error) {
	query := `
		SELECT id, alert_type, level, title, message, symbol, metadata, channels, read, created_at
		FROM alert_records ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询提醒记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []*storage.AlertRecord
	for rows.Next() {
		record := &storage.AlertRecord{}
		err := rows.Scan(
			&record.ID, &record.AlertType, &record.Level, &record.Title, &record.Message,
			&record.Symbol, &record.Metadata, &record.Channels, &record.Read, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描提醒记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历提醒记录失败: %w", err)
	}

	return records, nil
}

// ListUnread 列出未读提醒记录
func (r *alertRecordRepository) ListUnread(limit, offset int) ([]*storage.AlertRecord, error) {
	query := `
		SELECT id, alert_type, level, title, message, symbol, metadata, channels, read, created_at
		FROM alert_records WHERE read = 0
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询未读提醒记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []*storage.AlertRecord
	for rows.Next() {
		record := &storage.AlertRecord{}
		err := rows.Scan(
			&record.ID, &record.AlertType, &record.Level, &record.Title, &record.Message,
			&record.Symbol, &record.Metadata, &record.Channels, &record.Read, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描未读提醒记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历未读提醒记录失败: %w", err)
	}

	return records, nil
}

// MarkAsRead 标记提醒为已读
func (r *alertRecordRepository) MarkAsRead(id int64) error {
	query := `UPDATE alert_records SET read = 1 WHERE id = ?`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("标记提醒为已读失败: %w", err)
	}

	return nil
}

// ListRecent 获取最近的提醒
func (r *alertRecordRepository) ListRecent(limit int) ([]*storage.AlertRecord, error) {
	query := `
		SELECT id, alert_type, level, title, message, symbol, metadata, channels, read, created_at
		FROM alert_records ORDER BY created_at DESC LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询最近提醒记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []*storage.AlertRecord
	for rows.Next() {
		record := &storage.AlertRecord{}
		err := rows.Scan(
			&record.ID, &record.AlertType, &record.Level, &record.Title, &record.Message,
			&record.Symbol, &record.Metadata, &record.Channels, &record.Read, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描提醒记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历提醒记录失败: %w", err)
	}

	return records, nil
}

// ListByType 根据类型获取提醒
func (r *alertRecordRepository) ListByType(alertType string, limit, offset int) ([]*storage.AlertRecord, error) {
	query := `
		SELECT id, alert_type, level, title, message, symbol, metadata, channels, read, created_at
		FROM alert_records WHERE alert_type = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, alertType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询类型提醒记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []*storage.AlertRecord
	for rows.Next() {
		record := &storage.AlertRecord{}
		err := rows.Scan(
			&record.ID, &record.AlertType, &record.Level, &record.Title, &record.Message,
			&record.Symbol, &record.Metadata, &record.Channels, &record.Read, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描提醒记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历提醒记录失败: %w", err)
	}

	return records, nil
}
