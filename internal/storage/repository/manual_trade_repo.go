package repository

import (
	"database/sql"
	"fmt"

	"github.com/ljwqf/quant/internal/storage"
)

// ManualTradeRepository 手动交易数据访问接口
type ManualTradeRepository interface {
	Create(trade *storage.ManualTrade) error
	GetByID(id int64) (*storage.ManualTrade, error)
	GetByOrderID(orderID string) (*storage.ManualTrade, error)
	List(symbol string, limit, offset int) ([]*storage.ManualTrade, error)
	Update(trade *storage.ManualTrade) error
	Delete(id int64) error
}

// manualTradeRepository 手动交易数据访问实现
type manualTradeRepository struct {
	db *sql.DB
}

// NewManualTradeRepository 创建手动交易数据访问
func NewManualTradeRepository(db *sql.DB) ManualTradeRepository {
	return &manualTradeRepository{db: db}
}

// Create 创建手动交易记录
func (r *manualTradeRepository) Create(trade *storage.ManualTrade) error {
	query := `
		INSERT INTO manual_trades (
			order_id, symbol, side, type, price, size, filled_size, status,
			leverage, take_profit, stop_loss, ai_analysis_id, ai_analysis_summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		trade.OrderID, trade.Symbol, trade.Side, trade.Type,
		trade.Price, trade.Size, trade.FilledSize, trade.Status,
		trade.Leverage, trade.TakeProfit, trade.StopLoss,
		trade.AIAnalysisID, trade.AIAnalysisSummary,
	)
	if err != nil {
		return fmt.Errorf("创建手动交易记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}

	trade.ID = id
	return nil
}

// GetByID 根据ID获取手动交易记录
func (r *manualTradeRepository) GetByID(id int64) (*storage.ManualTrade, error) {
	query := `
		SELECT id, order_id, symbol, side, type, price, size, filled_size, status,
			leverage, take_profit, stop_loss, ai_analysis_id, ai_analysis_summary,
			created_at, updated_at
		FROM manual_trades WHERE id = ?
	`

	trade := &storage.ManualTrade{}
	err := r.db.QueryRow(query, id).Scan(
		&trade.ID, &trade.OrderID, &trade.Symbol, &trade.Side, &trade.Type,
		&trade.Price, &trade.Size, &trade.FilledSize, &trade.Status,
		&trade.Leverage, &trade.TakeProfit, &trade.StopLoss,
		&trade.AIAnalysisID, &trade.AIAnalysisSummary,
		&trade.CreatedAt, &trade.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询手动交易记录失败: %w", err)
	}

	return trade, nil
}

// GetByOrderID 根据订单ID获取手动交易记录
func (r *manualTradeRepository) GetByOrderID(orderID string) (*storage.ManualTrade, error) {
	query := `
		SELECT id, order_id, symbol, side, type, price, size, filled_size, status,
			leverage, take_profit, stop_loss, ai_analysis_id, ai_analysis_summary,
			created_at, updated_at
		FROM manual_trades WHERE order_id = ?
	`

	trade := &storage.ManualTrade{}
	err := r.db.QueryRow(query, orderID).Scan(
		&trade.ID, &trade.OrderID, &trade.Symbol, &trade.Side, &trade.Type,
		&trade.Price, &trade.Size, &trade.FilledSize, &trade.Status,
		&trade.Leverage, &trade.TakeProfit, &trade.StopLoss,
		&trade.AIAnalysisID, &trade.AIAnalysisSummary,
		&trade.CreatedAt, &trade.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询手动交易记录失败: %w", err)
	}

	return trade, nil
}

// List 列出手动交易记录
func (r *manualTradeRepository) List(symbol string, limit, offset int) ([]*storage.ManualTrade, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, order_id, symbol, side, type, price, size, filled_size, status,
				leverage, take_profit, stop_loss, ai_analysis_id, ai_analysis_summary,
				created_at, updated_at
			FROM manual_trades WHERE symbol = ?
			ORDER BY created_at DESC LIMIT ? OFFSET ?
		`
		args = []interface{}{symbol, limit, offset}
	} else {
		query = `
			SELECT id, order_id, symbol, side, type, price, size, filled_size, status,
				leverage, take_profit, stop_loss, ai_analysis_id, ai_analysis_summary,
				created_at, updated_at
			FROM manual_trades
			ORDER BY created_at DESC LIMIT ? OFFSET ?
		`
		args = []interface{}{limit, offset}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询手动交易记录列表失败: %w", err)
	}
	defer rows.Close()

	var trades []*storage.ManualTrade
	for rows.Next() {
		trade := &storage.ManualTrade{}
		err := rows.Scan(
			&trade.ID, &trade.OrderID, &trade.Symbol, &trade.Side, &trade.Type,
			&trade.Price, &trade.Size, &trade.FilledSize, &trade.Status,
			&trade.Leverage, &trade.TakeProfit, &trade.StopLoss,
			&trade.AIAnalysisID, &trade.AIAnalysisSummary,
			&trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描手动交易记录失败: %w", err)
		}
		trades = append(trades, trade)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历手动交易记录失败: %w", err)
	}

	return trades, nil
}

// Update 更新手动交易记录
func (r *manualTradeRepository) Update(trade *storage.ManualTrade) error {
	query := `
		UPDATE manual_trades
		SET order_id = ?, symbol = ?, side = ?, type = ?, price = ?, size = ?,
			filled_size = ?, status = ?, leverage = ?, take_profit = ?, stop_loss = ?,
			ai_analysis_id = ?, ai_analysis_summary = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		trade.OrderID, trade.Symbol, trade.Side, trade.Type,
		trade.Price, trade.Size, trade.FilledSize, trade.Status,
		trade.Leverage, trade.TakeProfit, trade.StopLoss,
		trade.AIAnalysisID, trade.AIAnalysisSummary, trade.ID,
	)
	if err != nil {
		return fmt.Errorf("更新手动交易记录失败: %w", err)
	}

	return nil
}

// Delete 删除手动交易记录
func (r *manualTradeRepository) Delete(id int64) error {
	query := `DELETE FROM manual_trades WHERE id = ?`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("删除手动交易记录失败: %w", err)
	}

	return nil
}
