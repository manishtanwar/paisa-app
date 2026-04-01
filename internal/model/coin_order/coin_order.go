package coin_order

import (
	"time"

	"gorm.io/gorm"
)

// CoinOrder stores Coin (Kite MF) orders to prevent duplicate ledger entries.
// PK is order_id — each MF order is atomic and uniquely identified by it.
type CoinOrder struct {
	OrderID           string    `gorm:"primaryKey" json:"order_id"`
	ExchangeOrderID   string    `json:"exchange_order_id"`
	TradingSymbol     string    `json:"tradingsymbol"`
	Status            string    `json:"status"`
	StatusMessage     string    `json:"status_message"`
	Fund              string    `json:"fund"`
	Folio             string    `json:"folio"`
	OrderTimestamp    time.Time `json:"order_timestamp"`
	ExchangeTimestamp string    `json:"exchange_timestamp"`
	SettlementID      string    `json:"settlement_id"`
	TransactionType   string    `json:"transaction_type"`
	Amount            string    `json:"amount"`
	Variety           string    `json:"variety"`
	PurchaseType      string    `json:"purchase_type"`
	Quantity          string    `json:"quantity"`
	Price             string    `json:"price"`
	LastPrice         string    `json:"last_price"`
	AveragePrice      string    `json:"average_price"`
	PlacedBy          string    `json:"placed_by"`
	LastPriceDate     string    `json:"last_price_date"`
	Tag               string    `json:"tag"`
	CreatedAt         time.Time `json:"created_at"`
}

func (CoinOrder) TableName() string {
	return "coin_orders"
}

// Exists returns true if an order with the given order_id is already stored.
func Exists(db *gorm.DB, orderID string) (bool, error) {
	var count int64
	err := db.Model(&CoinOrder{}).Where("order_id = ?", orderID).Count(&count).Error
	return count > 0, err
}

// Create inserts a new order record.
func Create(db *gorm.DB, o *CoinOrder) error {
	o.CreatedAt = time.Now()
	return db.Create(o).Error
}
