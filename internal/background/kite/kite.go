package kite

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/prediction"
)

// KiteTime is a custom time type that can handle KITE API timestamp format
type KiteTime struct {
	time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling for KITE API timestamp format
func (kt *KiteTime) UnmarshalJSON(data []byte) error {
	// Remove quotes from the JSON string
	str := strings.Trim(string(data), `"`)

	if str == "" || str == "null" {
		kt.Time = time.Time{}
		return nil
	}

	// Parse the specific KITE API format: "2021-05-31 16:00:36"
	t, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil {
		return fmt.Errorf("unable to parse KITE timestamp %s: %w", str, err)
	}

	kt.Time = t
	return nil
}

// Trade represents a trade from KITE Connect API
type Trade struct {
	TradeID           string          `json:"trade_id"`
	OrderID           string          `json:"order_id"`
	ExchangeOrderID   string          `json:"exchange_order_id"`
	TradingSymbol     string          `json:"tradingsymbol"`
	Exchange          string          `json:"exchange"`
	TransactionType   string          `json:"transaction_type"` // BUY or SELL
	Product           string          `json:"product"`
	AveragePrice      decimal.Decimal `json:"average_price"`
	Quantity          int             `json:"quantity"`
	FillTimestamp     KiteTime        `json:"fill_timestamp"`
	ExchangeTimestamp KiteTime        `json:"exchange_timestamp"`
}

type DailyTradesTask struct{}

func (t *DailyTradesTask) Name() string {
	return "Daily Trades Fetch"
}

func (t *DailyTradesTask) Schedule() string {
	return "0 16 * * *" // Run at 4 PM daily
}

func (t *DailyTradesTask) ShouldRunOnStartup() bool {
	return true
}

func (t *DailyTradesTask) Run(ctx context.Context, db *gorm.DB) error {
	log.Info("Starting daily trades fetch from KITE Connect for all accounts")

	// Load KITE configuration
	kiteConfig, err := loadKiteConfig()
	if err != nil {
		return fmt.Errorf("failed to load KITE config: %w", err)
	}

	if len(kiteConfig.Accounts) == 0 {
		return fmt.Errorf("no KITE accounts configured")
	}

	// Process each account
	for _, account := range kiteConfig.Accounts {
		log.Infof("Processing account: %s", account.Name)

		// Get a valid access token for this account
		accessToken, err := GetValidAccessToken(db, account.APIKey)
		if err != nil {
			log.Warnf("Failed to get a valid access token for account %s: %v", account.Name, err)
			continue // Continue with other accounts even if one fails
		}

		log.Infof("Successfully authenticated with KITE Connect for account: %s", account.Name)

		// Fetch trades for today for this account
		trades, err := fetchDailyTrades(ctx, account.APIKey, accessToken)
		log.Infof("Fetched %d trades for account %s %s", len(trades), account.APIKey, accessToken)
		if err != nil {
			log.Warnf("Failed to fetch daily trades for account %s: %v", account.Name, err)
			continue // Continue with other accounts even if one fails
		}

		log.Infof("Found %d trades for account %s", len(trades), account.Name)

		// Convert trades to ledger format and save
		err = saveTradesToLedger(db, account.Name, account.LedgerFile, trades, time.Now().Format("2006-01-02"))
		if err != nil {
			return fmt.Errorf("failed to save trades to ledger: %w", err)
		}

		log.Infof("Successfully processed %d trades for account %s", len(trades), account.Name)

		// Fetch and save Coin (mutual fund) transactions
		mfOrders, err := fetchCoinTransactions(ctx, account.APIKey, accessToken)
		if err != nil {
			log.Warnf("Failed to fetch Coin MF transactions for account %s: %v", account.Name, err)
			continue
		}

		log.Infof("Found %d Coin MF orders for account %s", len(mfOrders), account.Name)

		err = saveCoinTransactionsToLedger(db, account.Name, account.CoinLedgerFile, mfOrders, time.Now().Format("2006-01-02"))
		if err != nil {
			return fmt.Errorf("failed to save Coin MF transactions to ledger: %w", err)
		}

		log.Infof("Successfully processed Coin MF orders for account %s", account.Name)
	}

	return nil
}

// loadKiteConfig loads KITE Connect configuration from the config directory
func loadKiteConfig() (*KiteConfig, error) {
	configDir := config.GetConfigDir()
	kiteConfigPath := filepath.Join(configDir, "kite.yaml")

	// Check if config file exists
	if _, err := os.Stat(kiteConfigPath); os.IsNotExist(err) {
		templateConfig := &KiteConfig{
			Accounts: []KiteAccount{
				{
					Name:           "Primary Account",
					APIKey:         "your_api_key_here",
					APISecret:      "your_api_secret_here",
					UserID:         "your_user_id_here",
					Password:       "your_password_here",
					TOTPToken:      "your_totp_secret_here",
					LedgerFile:     "",
					CoinLedgerFile: "",
				},
				{
					Name:           "Secondary Account",
					APIKey:         "your_second_api_key_here",
					APISecret:      "your_second_api_secret_here",
					UserID:         "your_second_user_id_here",
					Password:       "your_second_password_here",
					TOTPToken:      "your_second_totp_secret_here",
					LedgerFile:     "",
					CoinLedgerFile: "",
				},
			},
		}

		// Generate proper YAML
		yamlData, err := yaml.Marshal(templateConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal template config: %w", err)
		}

		err = os.WriteFile(kiteConfigPath, yamlData, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to create template config file: %w", err)
		}

		log.Infof("Created template KITE config file at: %s", kiteConfigPath)
		log.Info("Please update the configuration with your KITE Connect credentials")
		return nil, fmt.Errorf("KITE config file created, please update with your credentials")
	}

	// Read existing config
	configData, err := os.ReadFile(kiteConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read KITE config file: %w", err)
	}

	var kiteConfig KiteConfig
	err = yaml.Unmarshal(configData, &kiteConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse KITE config file: %w", err)
	}

	return &kiteConfig, nil
}

// fetchDailyTrades fetches trades for a specific date from KITE Connect API
func fetchDailyTrades(ctx context.Context, apiKey string, accessToken string) ([]Trade, error) {
	// KITE Connect API endpoint for fetching trades
	url := "https://api.kite.trade/trades"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
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

	// Parse the response
	var response struct {
		Status string  `json:"status"`
		Data   []Trade `json:"data"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("API returned non-success status: %s", response.Status)
	}

	return response.Data, nil
}

// saveTradesToLedger converts trades to ledger format and saves them.
// If ledgerFile is non-empty, trades are appended to that file; otherwise the default journal is used.
func saveTradesToLedger(db *gorm.DB, accountName string, ledgerFile string, trades []Trade, date string) error {
	journalPath := config.GetJournalPath()
	if ledgerFile != "" {
		journalPath = ledgerFile
	}

	// Read existing journal content
	journalContent, err := os.ReadFile(journalPath)
	if err != nil {
		return fmt.Errorf("failed to read journal file: %w", err)
	}

	commentTime := time.Now().Format("3:04 PM")

	// Generate ledger entries for trades
	var ledgerEntries []string
	for _, trade := range trades {
		entry := generateLedgerEntry(db, trade)
		if entry != "" {
			// Add comment with date, time and account name before each entry
			commentedEntry := fmt.Sprintf("\n; Auto added on %s %s - %s\n%s", date, commentTime, accountName, entry)
			ledgerEntries = append(ledgerEntries, commentedEntry)
		}
	}

	if len(ledgerEntries) == 0 {
		log.Info("No valid ledger entries generated from trades")
		return nil
	}

	// Join entries with double newlines for better readability
	tradeSection := "\n" + strings.Join(ledgerEntries, "\n\n") + "\n"

	// Append to journal file and prettify
	updatedContent := ledger.FormatContent(string(journalContent) + tradeSection)
	err = os.WriteFile(journalPath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated journal file: %w", err)
	}

	log.Infof("Added %d trade entries to journal file", len(ledgerEntries))
	return nil
}

// generateLedgerEntry converts a trade to ledger format, using predictAccount to
// resolve the asset account name from the existing journal's posting history.
func generateLedgerEntry(db *gorm.DB, trade Trade) string {
	// Use the actual trade timestamp from the API
	tradeDate := trade.FillTimestamp.Time

	// Predict the account using the trading symbol as the query
	assetAccount := prediction.PredictAccount(db, trade.TradingSymbol, "Assets")

	// Determine transaction type and quantity
	quantity := trade.Quantity
	description := ""

	switch trade.TransactionType {
	case "BUY":
		description = fmt.Sprintf("Purchased %d Shares of %s", quantity, trade.TradingSymbol)
	case "SELL":
		quantity = -quantity
		description = fmt.Sprintf("Sold %d Shares of %s", trade.Quantity, trade.TradingSymbol)
	default:
		log.Warnf("Unknown transaction type: %s", trade.TransactionType)
		return ""
	}

	// Format the price with 4 decimal places
	price := trade.AveragePrice.Round(4)

	// Generate ledger entry
	entry := fmt.Sprintf("%s %s\n", tradeDate.Format("2006/01/02"), description)
	entry += fmt.Sprintf("    %s\t\t\t%d \"%s\" @ %s INR\n",
		assetAccount, quantity, assetAccount, price.String())
	entry += "    Assets:Checking:Broker:Kite"

	return entry
}

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

	var ledgerEntries []string
	for _, order := range orders {
		entry := generateMFLedgerEntry(db, order)
		if entry != "" {
			commentedEntry := fmt.Sprintf("\n; Auto added on %s %s - %s\n%s", date, commentTime, accountName, entry)
			ledgerEntries = append(ledgerEntries, commentedEntry)
		}
	}

	if len(ledgerEntries) == 0 {
		log.Info("No valid ledger entries generated from Coin MF orders")
		return nil
	}

	tradeSection := "\n" + strings.Join(ledgerEntries, "\n\n") + "\n"

	// Append to journal file and prettify
	updatedContent := ledger.FormatContent(string(journalContent) + tradeSection)
	err = os.WriteFile(journalPath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated journal file: %w", err)
	}

	log.Infof("Added %d Coin MF entries to journal file", len(ledgerEntries))
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
