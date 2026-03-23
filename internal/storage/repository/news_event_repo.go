package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// NewsEventRepository 新闻事件数据访问接口
type NewsEventRepository interface {
	Create(event *storage.NewsEvent) error
	Upsert(event *storage.NewsEvent) error
	GetByID(id int64) (*storage.NewsEvent, error)
	GetByExternalID(externalID string) (*storage.NewsEvent, error)
	List(limit, offset int) ([]*storage.NewsEvent, error)
	ListByImportance(minImportance int, limit, offset int) ([]*storage.NewsEvent, error)
}

// newsEventRepository 新闻事件数据访问实现
type newsEventRepository struct {
	db *sql.DB
}

// NewNewsEventRepository 创建新闻事件数据访问
func NewNewsEventRepository(db *sql.DB) NewsEventRepository {
	return &newsEventRepository{db: db}
}

// Create 创建新闻事件记录
func (r *newsEventRepository) Create(event *storage.NewsEvent) error {
	query := `
		INSERT INTO news_events (
			external_id, title, summary, content, source, url, image_url, category,
			tags, importance, sentiment, related_symbols, published_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		event.ExternalID, event.Title, event.Summary, event.Content,
		event.Source, event.URL, event.ImageURL, event.Category,
		event.Tags, event.Importance, event.Sentiment, event.RelatedSymbols,
		event.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("创建新闻事件记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}

	event.ID = id
	return nil
}

// Upsert 更新或插入新闻事件记录
func (r *newsEventRepository) Upsert(event *storage.NewsEvent) error {
	existing, err := r.GetByExternalID(event.ExternalID)
	if err != nil {
		return err
	}

	if existing != nil {
		event.ID = existing.ID
		return r.update(event)
	}
	return r.Create(event)
}

func (r *newsEventRepository) update(event *storage.NewsEvent) error {
	query := `
		UPDATE news_events
		SET title = ?, summary = ?, content = ?, source = ?, url = ?, image_url = ?,
			category = ?, tags = ?, importance = ?, sentiment = ?, related_symbols = ?,
			published_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		event.Title, event.Summary, event.Content, event.Source,
		event.URL, event.ImageURL, event.Category, event.Tags,
		event.Importance, event.Sentiment, event.RelatedSymbols,
		event.PublishedAt, event.ID,
	)
	if err != nil {
		return fmt.Errorf("更新新闻事件记录失败: %w", err)
	}

	return nil
}

// GetByID 根据ID获取新闻事件记录
func (r *newsEventRepository) GetByID(id int64) (*storage.NewsEvent, error) {
	query := `
		SELECT id, external_id, title, summary, content, source, url, image_url,
			category, tags, importance, sentiment, related_symbols, published_at, created_at
		FROM news_events WHERE id = ?
	`

	event := &storage.NewsEvent{}
	err := r.db.QueryRow(query, id).Scan(
		&event.ID, &event.ExternalID, &event.Title, &event.Summary, &event.Content,
		&event.Source, &event.URL, &event.ImageURL, &event.Category, &event.Tags,
		&event.Importance, &event.Sentiment, &event.RelatedSymbols,
		&event.PublishedAt, &event.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询新闻事件记录失败: %w", err)
	}

	return event, nil
}

// GetByExternalID 根据外部ID获取新闻事件记录
func (r *newsEventRepository) GetByExternalID(externalID string) (*storage.NewsEvent, error) {
	query := `
		SELECT id, external_id, title, summary, content, source, url, image_url,
			category, tags, importance, sentiment, related_symbols, published_at, created_at
		FROM news_events WHERE external_id = ?
	`

	event := &storage.NewsEvent{}
	err := r.db.QueryRow(query, externalID).Scan(
		&event.ID, &event.ExternalID, &event.Title, &event.Summary, &event.Content,
		&event.Source, &event.URL, &event.ImageURL, &event.Category, &event.Tags,
		&event.Importance, &event.Sentiment, &event.RelatedSymbols,
		&event.PublishedAt, &event.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询新闻事件记录失败: %w", err)
	}

	return event, nil
}

// List 列出新闻事件记录
func (r *newsEventRepository) List(limit, offset int) ([]*storage.NewsEvent, error) {
	query := `
		SELECT id, external_id, title, summary, content, source, url, image_url,
			category, tags, importance, sentiment, related_symbols, published_at, created_at
		FROM news_events ORDER BY published_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询新闻事件记录列表失败: %w", err)
	}
	defer rows.Close()

	var events []*storage.NewsEvent
	for rows.Next() {
		event := &storage.NewsEvent{}
		err := rows.Scan(
			&event.ID, &event.ExternalID, &event.Title, &event.Summary, &event.Content,
			&event.Source, &event.URL, &event.ImageURL, &event.Category, &event.Tags,
			&event.Importance, &event.Sentiment, &event.RelatedSymbols,
			&event.PublishedAt, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描新闻事件记录失败: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历新闻事件记录失败: %w", err)
	}

	return events, nil
}

// ListByImportance 根据重要程度列出新闻事件记录
func (r *newsEventRepository) ListByImportance(minImportance int, limit, offset int) ([]*storage.NewsEvent, error) {
	query := `
		SELECT id, external_id, title, summary, content, source, url, image_url,
			category, tags, importance, sentiment, related_symbols, published_at, created_at
		FROM news_events WHERE importance >= ?
		ORDER BY published_at DESC LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, minImportance, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询新闻事件记录列表失败: %w", err)
	}
	defer rows.Close()

	var events []*storage.NewsEvent
	for rows.Next() {
		event := &storage.NewsEvent{}
		err := rows.Scan(
			&event.ID, &event.ExternalID, &event.Title, &event.Summary, &event.Content,
			&event.Source, &event.URL, &event.ImageURL, &event.Category, &event.Tags,
			&event.Importance, &event.Sentiment, &event.RelatedSymbols,
			&event.PublishedAt, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描新闻事件记录失败: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历新闻事件记录失败: %w", err)
	}

	return events, nil
}
