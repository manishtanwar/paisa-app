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
)

// KiteTime is a custom time type that can handle KITE API timestamp format
type KiteTime struct {
	time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling for KITE API timestamp format
func (kt *KiteTime) UnmarshalJSON(data []byte) error {
	// Remove quotes from the JSON string
	str := strings.Trim(string(data), `"`)

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
		err = saveTradesToLedger(account.Name, trades, time.Now().Format("2006-01-02"))
		if err != nil {
			return fmt.Errorf("failed to save trades to ledger: %w", err)
		}

		log.Infof("Successfully processed %d trades for account %s", len(trades), account.Name)
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
					Name:      "Primary Account",
					APIKey:    "your_api_key_here",
					APISecret: "your_api_secret_here",
					UserID:    "your_user_id_here",
					Password:  "your_password_here",
					TOTPToken: "your_totp_secret_here",
				},
				{
					Name:      "Secondary Account",
					APIKey:    "your_second_api_key_here",
					APISecret: "your_second_api_secret_here",
					UserID:    "your_second_user_id_here",
					Password:  "your_second_password_here",
					TOTPToken: "your_second_totp_secret_here",
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

// saveTradesToLedger converts trades to ledger format and saves them
func saveTradesToLedger(accountName string, trades []Trade, date string) error {
	journalPath := config.GetJournalPath()

	// Read existing journal content
	journalContent, err := os.ReadFile(journalPath)
	if err != nil {
		return fmt.Errorf("failed to read journal file: %w", err)
	}

	commentTime := time.Now().Format("3:04 PM")

	// Generate ledger entries for trades
	var ledgerEntries []string
	for _, trade := range trades {
		entry := generateLedgerEntry(trade)
		if entry != "" {
			// Add comment with date, time and account name before each entry
			commentedEntry := fmt.Sprintf("; Auto added on %s %s - %s \n%s", date, commentTime, accountName, entry)
			ledgerEntries = append(ledgerEntries, commentedEntry)
		}
	}

	if len(ledgerEntries) == 0 {
		log.Info("No valid ledger entries generated from trades")
		return nil
	}

	// Join entries with double newlines for better readability
	tradeSection := "\n" + strings.Join(ledgerEntries, "\n\n") + "\n"

	// Append to journal file
	updatedContent := string(journalContent) + tradeSection
	err = os.WriteFile(journalPath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated journal file: %w", err)
	}

	log.Infof("Added %d trade entries to journal file", len(ledgerEntries))
	return nil
}

// generateLedgerEntry converts a trade to ledger format
func generateLedgerEntry(trade Trade) string {
	// Use the actual trade timestamp from the API
	tradeDate := trade.FillTimestamp.Time

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
	entry += fmt.Sprintf("    Assets:Equity:Stocks:%s\t\t\t%d \"%s\" @ %s INR\n",
		trade.TradingSymbol, quantity, trade.TradingSymbol, price.String())
	entry += "    Assets:Checking:Broker:Zerodha"

	return entry
}
