package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// EconomicEventRepository 经济事件数据访问接口
type EconomicEventRepository interface {
	Create(event *storage.EconomicEvent) error
	GetByID(id int64) (*storage.EconomicEvent, error)
	List(limit, offset int) ([]*storage.EconomicEvent, error)
	ListUpcoming(days int) ([]*storage.EconomicEvent, error)
}

// economicEventRepository 经济事件数据访问实现
type economicEventRepository struct {
	db *sql.DB
}

// NewEconomicEventRepository 创建经济事件数据访问
func NewEconomicEventRepository(db *sql.DB) EconomicEventRepository {
	return &economicEventRepository{db: db}
}

// Create 创建经济事件记录
func (r *economicEventRepository) Create(event *storage.EconomicEvent) error {
	query := `
		INSERT INTO economic_events (
			external_id, title, country, currency, indicator, actual, forecast,
			previous, unit, importance, impact, event_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		event.ExternalID, event.Title, event.Country, event.Currency,
		event.Indicator, event.Actual, event.Forecast, event.Previous,
		event.Unit, event.Importance, event.Impact, event.EventTime,
	)
	if err != nil {
		return fmt.Errorf("创建经济事件记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}

	event.ID = id
	return nil
}

// GetByID 根据ID获取经济事件记录
func (r *economicEventRepository) GetByID(id int64) (*storage.EconomicEvent, error) {
	query := `
		SELECT id, external_id, title, country, currency, indicator, actual, forecast,
			previous, unit, importance, impact, event_time, created_at
		FROM economic_events WHERE id = ?
	`

	event := &storage.EconomicEvent{}
	err := r.db.QueryRow(query, id).Scan(
		&event.ID, &event.ExternalID, &event.Title, &event.Country, &event.Currency,
		&event.Indicator, &event.Actual, &event.Forecast, &event.Previous,
		&event.Unit, &event.Importance, &event.Impact, &event.EventTime, &event.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询经济事件记录失败: %w", err)
	}

	return event, nil
}

// List 列出经济事件记录
func (r *economicEventRepository) List(limit, offset int) ([]*storage.EconomicEvent, error) {
	query := `
		SELECT id, external_id, title, country, currency, indicator, actual, forecast,
			previous, unit, importance, impact, event_time, created_at
		FROM economic_events ORDER BY event_time DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询经济事件记录列表失败: %w", err)
	}
	defer rows.Close()

	var events []*storage.EconomicEvent
	for rows.Next() {
		event := &storage.EconomicEvent{}
		err := rows.Scan(
			&event.ID, &event.ExternalID, &event.Title, &event.Country, &event.Currency,
			&event.Indicator, &event.Actual, &event.Forecast, &event.Previous,
			&event.Unit, &event.Importance, &event.Impact, &event.EventTime, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描经济事件记录失败: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历经济事件记录失败: %w", err)
	}

	return events, nil
}

// ListUpcoming 获取即将到来的经济事件
func (r *economicEventRepository) ListUpcoming(days int) ([]*storage.EconomicEvent, error) {
	query := `
		SELECT id, external_id, title, country, currency, indicator, actual, forecast,
			previous, unit, importance, impact, event_time, created_at
		FROM economic_events 
		WHERE event_time >= datetime('now')
		AND event_time <= datetime('now', '+' || ? || ' days')
		ORDER BY event_time ASC
	`

	rows, err := r.db.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("查询即将到来的经济事件失败: %w", err)
	}
	defer rows.Close()

	var events []*storage.EconomicEvent
	for rows.Next() {
		event := &storage.EconomicEvent{}
		err := rows.Scan(
			&event.ID, &event.ExternalID, &event.Title, &event.Country, &event.Currency,
			&event.Indicator, &event.Actual, &event.Forecast, &event.Previous,
			&event.Unit, &event.Importance, &event.Impact, &event.EventTime, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描经济事件记录失败: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历经济事件记录失败: %w", err)
	}

	return events, nil
}
