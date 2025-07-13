package prices

import (
	"context"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/model"
)

// DailyPriceUpdateTask implements the background task for updating daily prices
type DailyPriceUpdateTask struct{}

func (t *DailyPriceUpdateTask) Name() string {
	return "Daily Price Update"
}

func (t *DailyPriceUpdateTask) Schedule() string {
	return "0 18 * * *" // Run at 6 PM daily (after market hours)
}

func (t *DailyPriceUpdateTask) ShouldRunOnStartup() bool {
	return false // Should run on every startup
}

func (t *DailyPriceUpdateTask) Run(ctx context.Context, db *gorm.DB) error {
	log.Info("Starting daily price update")

	// Update commodity prices
	err := model.SyncCommodities(db)
	if err != nil {
		return err
	}

	// Update CII (Cost Inflation Index) for tax calculations
	err = model.SyncCII(db)
	if err != nil {
		log.Warnf("Failed to sync CII: %v", err)
		// Don't fail the entire task for CII sync failure
	}

	// Update mutual fund portfolios
	err = model.SyncPortfolios(db)
	if err != nil {
		log.Warnf("Failed to sync portfolios: %v", err)
		// Don't fail the entire task for portfolio sync failure
	}

	log.Info("Daily price update completed successfully")
	return nil
}
