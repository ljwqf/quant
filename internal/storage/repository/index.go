package repository

import (
	"github.com/ljwqf/quant/internal/storage"
)

type Repositories struct {
	ManualTrade     ManualTradeRepository
	AIAnalysis      AIAnalysisRepository
	NewsEvent       NewsEventRepository
	EconomicEvent   EconomicEventRepository
	AlertRecord     AlertRecordRepository
	Kline           KlineRepository
	Tick            TickRepository
}

func NewRepositories(db *storage.Database) *Repositories {
	return &Repositories{
		ManualTrade:   NewManualTradeRepository(db.DB()),
		AIAnalysis:    NewAIAnalysisRepository(db.DB()),
		NewsEvent:     NewNewsEventRepository(db.DB()),
		EconomicEvent: NewEconomicEventRepository(db.DB()),
		AlertRecord:   NewAlertRecordRepository(db.DB()),
		Kline:         NewKlineRepository(db.DB()),
		Tick:          NewTickRepository(db.DB()),
	}
}
