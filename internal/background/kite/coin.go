package kite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	coin_order "github.com/ananthakumaran/paisa/internal/model/coin_order"
	"github.com/ananthakumaran/paisa/internal/prediction"
)

// MFOrder represents a mutual fund order from Kite/Coin API
type MFOrder struct {
	OrderID           string          `json:"order_id"`
	ExchangeOrderID   string          `json:"exchange_order_id"` // null or string
	TradingSymbol     string          `json:"tradingsymbol"`     // ISIN of the fund
	Status            string          `json:"status"`            // null or string: COMPLETE, REJECTED, CANCELLED, OPEN
	StatusMessage     string          `json:"status_message"`    // null or string: human readable status
	Fund              string          `json:"fund"`
	Folio             string          `json:"folio"` // null or string: AMC folio number
	OrderTimestamp    KiteTime        `json:"order_timestamp"`
	ExchangeTimestamp string          `json:"exchange_timestamp"` // null for orders that don't reach exchange
	SettlementID      string          `json:"settlement_id"`
	TransactionType   string          `json:"transaction_type"` // BUY or SELL
	Amount            decimal.Decimal `json:"amount"`
	Variety           string          `json:"variety"`       // regular or sip
	PurchaseType      string          `json:"purchase_type"` // null or string: FRESH or ADDITIONAL (null for SELL)
	Quantity          decimal.Decimal `json:"quantity"`
	Price             decimal.Decimal `json:"price"`
	LastPrice         decimal.Decimal `json:"last_price"`
	AveragePrice      decimal.Decimal `json:"average_price"`
	PlacedBy          string          `json:"placed_by"`
	LastPriceDate     string          `json:"last_price_date"`
	Tag               string          `json:"tag"`
}

// fetchCoinTransactions fetches today's mutual fund orders from Kite/Coin API
func fetchCoinTransactions(ctx context.Context, apiKey string, accessToken string) ([]MFOrder, error) {
	url := "https://api.kite.trade/mf/orders"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Kite-Version", "3")
	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", apiKey, accessToken))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response struct {
		Status string    `json:"status"`
		Data   []MFOrder `json:"data"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("API returned non-success status: %s", response.Status)
	}

	// Filter to only complete orders placed today
	today := time.Now().Format("2006-01-02")
	var todayOrders []MFOrder
	for _, order := range response.Data {
		if order.Status == "COMPLETE" && order.OrderTimestamp.Time.Format("2006-01-02") == today {
			todayOrders = append(todayOrders, order)
		}
	}

	return todayOrders, nil
}

// saveCoinTransactionsToLedger converts MF orders to ledger format and saves them.
// If coinLedgerFile is non-empty, entries are appended to that file; otherwise the default journal is used.
// Orders already present in the DB (by order_id) are skipped to prevent duplicate entries.
func saveCoinTransactionsToLedger(db *gorm.DB, accountName string, coinLedgerFile string, orders []MFOrder, date string) error {
	if len(orders) == 0 {
		log.Info("No Coin MF orders to save")
		return nil
	}

	journalPath := config.GetJournalPath()
	if coinLedgerFile != "" {
		journalPath = coinLedgerFile
	}

	journalContent, err := os.ReadFile(journalPath)
	if err != nil {
		return fmt.Errorf("failed to read journal file: %w", err)
	}

	commentTime := time.Now().Format("3:04 PM")

	type pendingOrder struct {
		entry string
		order MFOrder
	}
	var pending []pendingOrder
	for _, order := range orders {
		exists, err := coin_order.Exists(db, order.OrderID)
		if err != nil {
			return fmt.Errorf("failed to check order existence: %w", err)
		}
		if exists {
			log.Infof("Skipping already-recorded Coin order %s (%s)", order.OrderID, order.Fund)
			continue
		}
		entry := generateMFLedgerEntry(db, order)
		if entry != "" {
			commentedEntry := fmt.Sprintf("\n; Auto added on %s %s - %s\n%s", date, commentTime, accountName, entry)
			pending = append(pending, pendingOrder{entry: commentedEntry, order: order})
		}
	}

	if len(pending) == 0 {
		log.Info("No new Coin MF entries to add")
		return nil
	}

	var entries []string
	for _, p := range pending {
		entries = append(entries, p.entry)
	}

	tradeSection := "\n" + strings.Join(entries, "\n\n") + "\n"

	// Append to journal file and prettify
	updatedContent := ledger.FormatContent(string(journalContent) + tradeSection)
	err = os.WriteFile(journalPath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated journal file: %w", err)
	}

	// Record orders in DB only after a successful ledger write
	for _, p := range pending {
		o := p.order
		record := &coin_order.CoinOrder{
			OrderID:           o.OrderID,
			ExchangeOrderID:   o.ExchangeOrderID,
			TradingSymbol:     o.TradingSymbol,
			Status:            o.Status,
			StatusMessage:     o.StatusMessage,
			Fund:              o.Fund,
			Folio:             o.Folio,
			OrderTimestamp:    o.OrderTimestamp.Time,
			ExchangeTimestamp: o.ExchangeTimestamp,
			SettlementID:      o.SettlementID,
			TransactionType:   o.TransactionType,
			Amount:            o.Amount.String(),
			Variety:           o.Variety,
			PurchaseType:      o.PurchaseType,
			Quantity:          o.Quantity.String(),
			Price:             o.Price.String(),
			LastPrice:         o.LastPrice.String(),
			AveragePrice:      o.AveragePrice.String(),
			PlacedBy:          o.PlacedBy,
			LastPriceDate:     o.LastPriceDate,
			Tag:               o.Tag,
		}
		if err := coin_order.Create(db, record); err != nil {
			log.Warnf("Failed to record Coin order %s in DB: %v", o.OrderID, err)
		}
	}

	log.Infof("Added %d Coin MF entries to journal file", len(pending))
	return nil
}

// generateMFLedgerEntry converts a mutual fund order to ledger format.
func generateMFLedgerEntry(db *gorm.DB, order MFOrder) string {
	orderDate := order.OrderTimestamp.Time

	// Use fund name for account prediction
	assetAccount := prediction.PredictAccount(db, order.Fund, "Assets")

	quantity := order.Quantity
	description := ""

	switch order.TransactionType {
	case "BUY":
		description = fmt.Sprintf("Purchased %s Units of %s", quantity.String(), order.Fund)
	case "SELL":
		quantity = quantity.Neg()
		description = fmt.Sprintf("Redeemed %s Units of %s", order.Quantity.String(), order.Fund)
	default:
		log.Warnf("Unknown MF transaction type: %s", order.TransactionType)
		return ""
	}

	price := order.AveragePrice.Round(4)

	entry := fmt.Sprintf("%s %s\n", orderDate.Format("2006/01/02"), description)
	entry += fmt.Sprintf("    %s\t\t\t%s \"%s\" @ %s INR\n",
		assetAccount, quantity.String(), assetAccount, price.String())
	entry += "    Assets:Checking:Broker:Coin"

	return entry
}
