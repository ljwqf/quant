package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// ActivePositionRepository 策略活跃持仓数据访问接口
type ActivePositionRepository interface {
	Upsert(position *storage.ActivePosition) error
	Delete(strategy, symbol string) error
	ListByStrategy(strategy string) ([]*storage.ActivePosition, error)
	ListAll() ([]*storage.ActivePosition, error)
}

type activePositionRepository struct {
	db *sql.DB
}

// NewActivePositionRepository 创建策略活跃持仓数据访问
func NewActivePositionRepository(db *sql.DB) ActivePositionRepository {
	return &activePositionRepository{db: db}
}

// Upsert 插入或更新持仓记录（基于 strategy+symbol 唯一索引）
func (r *activePositionRepository) Upsert(position *storage.ActivePosition) error {
	query := `
		INSERT INTO active_positions (strategy, symbol, side, size, entry_price, order_id)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(strategy, symbol) DO UPDATE SET
			side = excluded.side,
			size = excluded.size,
			entry_price = excluded.entry_price,
			order_id = excluded.order_id
	`
	_, err := r.db.Exec(query,
		position.Strategy, position.Symbol, position.Side,
		position.Size, position.EntryPrice, position.OrderID,
	)
	if err != nil {
		return fmt.Errorf("插入/更新活跃持仓记录失败: %w", err)
	}
	return nil
}

// Delete 删除持仓记录（平仓时调用）
func (r *activePositionRepository) Delete(strategy, symbol string) error {
	query := `DELETE FROM active_positions WHERE strategy = ? AND symbol = ?`
	_, err := r.db.Exec(query, strategy, symbol)
	if err != nil {
		return fmt.Errorf("删除活跃持仓记录失败: %w", err)
	}
	return nil
}

// ListByStrategy 查询指定策略的活跃持仓
func (r *activePositionRepository) ListByStrategy(strategy string) ([]*storage.ActivePosition, error) {
	query := `
		SELECT id, strategy, symbol, side, size, entry_price, order_id, created_at
		FROM active_positions WHERE strategy = ?
	`
	rows, err := r.db.Query(query, strategy)
	if err != nil {
		return nil, fmt.Errorf("查询活跃持仓列表失败: %w", err)
	}
	defer rows.Close()

	var positions []*storage.ActivePosition
	for rows.Next() {
		p := &storage.ActivePosition{}
		if err := rows.Scan(&p.ID, &p.Strategy, &p.Symbol, &p.Side, &p.Size, &p.EntryPrice, &p.OrderID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描活跃持仓记录失败: %w", err)
		}
		positions = append(positions, p)
	}
	return positions, nil
}

// ListAll 查询所有活跃持仓
func (r *activePositionRepository) ListAll() ([]*storage.ActivePosition, error) {
	query := `
		SELECT id, strategy, symbol, side, size, entry_price, order_id, created_at
		FROM active_positions
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询活跃持仓列表失败: %w", err)
	}
	defer rows.Close()

	var positions []*storage.ActivePosition
	for rows.Next() {
		p := &storage.ActivePosition{}
		if err := rows.Scan(&p.ID, &p.Strategy, &p.Symbol, &p.Side, &p.Size, &p.EntryPrice, &p.OrderID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描活跃持仓记录失败: %w", err)
		}
		positions = append(positions, p)
	}
	return positions, nil
}
