package kite_trade

import (
	"time"

	"gorm.io/gorm"
)

// KiteTrade stores Kite stock/ETF trades to prevent duplicate ledger entries.
// PK is trade_id because a single order can produce multiple partial fills.
type KiteTrade struct {
	TradeID           string    `gorm:"primaryKey" json:"trade_id"`
	OrderID           string    `json:"order_id"`
	ExchangeOrderID   string    `json:"exchange_order_id"`
	TradingSymbol     string    `json:"tradingsymbol"`
	Exchange          string    `json:"exchange"`
	TransactionType   string    `json:"transaction_type"`
	Product           string    `json:"product"`
	AveragePrice      string    `json:"average_price"`
	Quantity          int       `json:"quantity"`
	FillTimestamp     time.Time `json:"fill_timestamp"`
	ExchangeTimestamp time.Time `json:"exchange_timestamp"`
	CreatedAt         time.Time `json:"created_at"`
}

func (KiteTrade) TableName() string {
	return "kite_trades"
}

// Exists returns true if a trade with the given trade_id is already stored.
func Exists(db *gorm.DB, tradeID string) (bool, error) {
	var count int64
	err := db.Model(&KiteTrade{}).Where("trade_id = ?", tradeID).Count(&count).Error
	return count > 0, err
}

// Create inserts a new trade record.
func Create(db *gorm.DB, t *KiteTrade) error {
	t.CreatedAt = time.Now()
	return db.Create(t).Error
}
