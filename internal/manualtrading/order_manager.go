package manualtrading

import (
	"time"

	"github.com/ljwqf/quant/internal/config"
	"github.com/ljwqf/quant/internal/exchange"
	"github.com/ljwqf/quant/internal/storage"
	"github.com/ljwqf/quant/internal/storage/repository"
	"github.com/ljwqf/quant/pkg/logger"
	"github.com/ljwqf/quant/pkg/types"
	"go.uber.org/zap"
)

type OrderManager struct {
	cfg     *config.ManualTradingConfig
	db      *storage.Database
	tradeRepo repository.ManualTradeRepository
	exchange exchange.Exchange
}

func NewOrderManager(cfg *config.ManualTradingConfig, db *storage.Database, exchange exchange.Exchange) *OrderManager {
	return &OrderManager{
		cfg:       cfg,
		db:        db,
		tradeRepo: repository.NewManualTradeRepository(db.DB()),
		exchange:  exchange,
	}
}

func (om *OrderManager) CreateOrder(trade *storage.ManualTrade) error {
	logger.Info("创建手动交易订单",
		zap.String("symbol", trade.Symbol),
		zap.String("side", trade.Side),
		zap.Float64("size", trade.Size),
	)

	orderSide := types.OrderSide(trade.Side)
	orderType := types.OrderType(trade.Type)
	if orderType == "" {
		orderType = types.OrderTypeMarket
	}

	order := &types.Order{
		Symbol:   trade.Symbol,
		Side:     orderSide,
		Type:     orderType,
		Quantity: trade.Size,
		Price:    trade.Price,
	}

	result, err := om.exchange.PlaceOrder(order)
	if err != nil {
		logger.Error("下单失败", zap.Error(err))
		return err
	}

	trade.OrderID = result.OrderID
	trade.Status = "pending"
	trade.CreatedAt = time.Now()
	trade.UpdatedAt = time.Now()

	if err := om.tradeRepo.Create(trade); err != nil {
		logger.Error("保存手动交易记录失败", zap.Error(err))
		return err
	}

	logger.Info("手动交易订单创建成功", zap.String("order_id", trade.OrderID))
	return nil
}

func (om *OrderManager) CancelOrder(orderID string) error {
	logger.Info("撤销订单", zap.String("order_id", orderID))

	if err := om.exchange.CancelOrder(orderID); err != nil {
		logger.Error("撤单失败", zap.Error(err))
		return err
	}

	trade, err := om.tradeRepo.GetByOrderID(orderID)
	if err == nil && trade != nil {
		trade.Status = "cancelled"
		trade.UpdatedAt = time.Now()
		om.tradeRepo.Update(trade)
	}

	logger.Info("订单已撤销", zap.String("order_id", orderID))
	return nil
}

func (om *OrderManager) GetOrder(orderID string) (*storage.ManualTrade, error) {
	return om.tradeRepo.GetByOrderID(orderID)
}

func (om *OrderManager) ListOrders(symbol string, limit, offset int) ([]*storage.ManualTrade, error) {
	return om.tradeRepo.List(symbol, limit, offset)
}

func (om *OrderManager) UpdateOrderStatus(orderID string, status string) error {
	trade, err := om.tradeRepo.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if trade == nil {
		return nil
	}

	trade.Status = status
	trade.UpdatedAt = time.Now()
	return om.tradeRepo.Update(trade)
}
