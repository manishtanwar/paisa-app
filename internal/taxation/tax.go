package taxation

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/service"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var EQUITY_GRANDFATHER_DATE, DEBT_INDEXATION_REVOCATION_DATE, CII_START_DATE, RATE_CHANGE_DATE time.Time
var ONE_YEAR = time.Hour * 24 * 365
var THREE_YEAR = ONE_YEAR * 3
var TWO_YEAR = ONE_YEAR * 2

func init() {
	EQUITY_GRANDFATHER_DATE, _ = time.ParseInLocation("2006-01-02", "2018-02-01", config.TimeZone())
	DEBT_INDEXATION_REVOCATION_DATE, _ = time.ParseInLocation("2006-01-02", "2023-04-01", config.TimeZone())
	CII_START_DATE, _ = time.ParseInLocation("2006-01-02", "2001-03-31", config.TimeZone())

	// Union Budget 2024: for transfers on or after this date, equity LTCG/STCG
	// rates rose to 12.5%/20%, indexation (Section 48) was withdrawn for
	// other assets, and their long term holding period fell from 36 to 24 months.
	RATE_CHANGE_DATE, _ = time.ParseInLocation("2006-01-02", "2024-07-23", config.TimeZone())
}

type Tax struct {
	Gain             decimal.Decimal `json:"gain"`
	Taxable          decimal.Decimal `json:"taxable"`
	Slab             decimal.Decimal `json:"slab"`
	LongTerm         decimal.Decimal `json:"long_term"`
	ShortTerm        decimal.Decimal `json:"short_term"`
	LongTermTaxable  decimal.Decimal `json:"long_term_taxable"`
	ShortTermTaxable decimal.Decimal `json:"short_term_taxable"`
}

func Add(a, b Tax) Tax {
	return Tax{
		Gain:             a.Gain.Add(b.Gain),
		Taxable:          a.Taxable.Add(b.Taxable),
		LongTerm:         a.LongTerm.Add(b.LongTerm),
		ShortTerm:        a.ShortTerm.Add(b.ShortTerm),
		Slab:             a.Slab.Add(b.Slab),
		LongTermTaxable:  a.LongTermTaxable.Add(b.LongTermTaxable),
		ShortTermTaxable: a.ShortTermTaxable.Add(b.ShortTermTaxable),
	}
}

// pickRate returns oldRate for transfers before Budget 2024 (23 Jul 2024), and
// newRate for transfers on or after it.
func pickRate(sellDate time.Time, oldRate, newRate float64) decimal.Decimal {
	if sellDate.Before(RATE_CHANGE_DATE) {
		return decimal.NewFromFloat(oldRate)
	}
	return decimal.NewFromFloat(newRate)
}

// pickThreshold returns oldThreshold for transfers before Budget 2024, and
// newThreshold for transfers on or after it.
func pickThreshold(sellDate time.Time, oldThreshold, newThreshold time.Duration) time.Duration {
	if sellDate.Before(RATE_CHANGE_DATE) {
		return oldThreshold
	}
	return newThreshold
}

func Calculate(db *gorm.DB, quantity decimal.Decimal, commodity config.Commodity, purchasePrice decimal.Decimal, purchaseDate time.Time, sellPrice decimal.Decimal, sellDate time.Time) Tax {

	dateDiff := sellDate.Sub(purchaseDate)
	gain := sellPrice.Mul(quantity).Sub(purchasePrice.Mul(quantity))

	if (commodity.TaxCategory == config.Equity || commodity.TaxCategory == config.Equity65) && sellDate.Before(EQUITY_GRANDFATHER_DATE) {
		return Tax{Gain: gain, Taxable: decimal.Zero, ShortTerm: decimal.Zero, LongTerm: decimal.Zero, Slab: decimal.Zero}
	}

	if (commodity.TaxCategory == config.Equity || commodity.TaxCategory == config.Equity65) && purchaseDate.Before(EQUITY_GRANDFATHER_DATE) {
		purchasePrice = service.GetUnitPrice(db, commodity.Name, EQUITY_GRANDFATHER_DATE).Value
	}

	// Indexation (Section 48 cost inflation adjustment) was withdrawn for
	// transfers on or after 23 Jul 2024, so it only applies before that date.
	indexationAvailable := sellDate.Before(RATE_CHANGE_DATE)

	if commodity.TaxCategory == config.Debt && indexationAvailable && purchaseDate.Before(DEBT_INDEXATION_REVOCATION_DATE) && purchaseDate.After(CII_START_DATE) && dateDiff > THREE_YEAR {
		purchasePrice = purchasePrice.Mul(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(sellDate)))).Div(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(purchaseDate))))))
	}

	if commodity.TaxCategory == config.UnlistedEquity && indexationAvailable && purchaseDate.After(CII_START_DATE) && dateDiff > TWO_YEAR {
		purchasePrice = purchasePrice.Mul(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(sellDate)))).Div(decimal.NewFromInt(int64(cii.GetIndex(db, utils.FY(purchaseDate))))))
	}

	taxable := sellPrice.Mul(quantity).Sub(purchasePrice.Mul(quantity))
	shortTerm := decimal.Zero
	longTerm := decimal.Zero
	slab := decimal.Zero
	shortTermTaxable := decimal.Zero
	longTermTaxable := decimal.Zero

	if commodity.TaxCategory == config.Equity || commodity.TaxCategory == config.Equity65 {
		// Holding period threshold (12 months) for listed equity/equity-oriented
		// funds is unchanged by Budget 2024, only the rates went up.
		if dateDiff > ONE_YEAR {
			longTermTaxable = taxable
			longTerm = taxable.Mul(pickRate(sellDate, 0.10, 0.125))
		} else {
			shortTermTaxable = taxable
			shortTerm = taxable.Mul(pickRate(sellDate, 0.15, 0.20))
		}

	}

	if commodity.TaxCategory == config.Debt {
		if !purchaseDate.Before(DEBT_INDEXATION_REVOCATION_DATE) {
			// "Specified mutual fund" (Finance Act 2023): always deemed short
			// term and taxed at slab rate, regardless of holding period.
			shortTermTaxable = taxable
			slab = taxable
		} else {
			threshold := pickThreshold(sellDate, THREE_YEAR, TWO_YEAR)
			if dateDiff > threshold {
				longTermTaxable = taxable
				longTerm = taxable.Mul(pickRate(sellDate, 0.20, 0.125))
			} else {
				shortTermTaxable = taxable
				slab = taxable
			}
		}
	}

	if commodity.TaxCategory == config.Equity35 {
		threshold := pickThreshold(sellDate, THREE_YEAR, TWO_YEAR)
		if dateDiff > threshold {
			longTermTaxable = taxable
			longTerm = taxable.Mul(pickRate(sellDate, 0.20, 0.125))
		} else {
			shortTermTaxable = taxable
			slab = taxable
		}
	}

	if commodity.TaxCategory == config.UnlistedEquity {
		// Unlisted shares already had a 24 month long term threshold before
		// Budget 2024; only the rate/indexation changed.
		if dateDiff > TWO_YEAR {
			longTermTaxable = taxable
			longTerm = taxable.Mul(pickRate(sellDate, 0.20, 0.125))
		} else {
			shortTermTaxable = taxable
			slab = taxable
		}
	}

	if commodity.TaxCategory == config.GoldETF {
		if dateDiff > ONE_YEAR {
			longTermTaxable = taxable
			longTerm = taxable.Mul(decimal.NewFromFloat(0.125))
		} else {
			shortTermTaxable = taxable
			slab = taxable
		}
	}

	return Tax{
		Gain:             gain,
		Taxable:          taxable,
		ShortTerm:        shortTerm,
		LongTerm:         longTerm,
		Slab:             slab,
		ShortTermTaxable: shortTermTaxable,
		LongTermTaxable:  longTermTaxable,
	}
}
