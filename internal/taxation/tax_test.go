package taxation

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&cii.CII{}))

	ciis := []*cii.CII{}
	for year, index := range map[string]uint{
		utils.FY(date(2015, 6, 1)):  254,
		utils.FY(date(2020, 6, 1)):  301,
		utils.FY(date(2021, 6, 1)):  317,
		utils.FY(date(2022, 6, 1)):  331,
		utils.FY(date(2023, 6, 1)):  348,
		utils.FY(date(2024, 6, 1)):  363,
	} {
		ciis = append(ciis, &cii.CII{FinancialYear: year, CostInflationIndex: index})
	}
	cii.UpsertAll(db, ciis)
	return db
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, config.TimeZone())
}

func commodity(category config.TaxCategoryType) config.Commodity {
	return config.Commodity{Name: "test", TaxCategory: category}
}

func TestEquityRates(t *testing.T) {
	db := testDB(t)
	one := decimal.NewFromInt(1)

	// Short term, before rate change: 15%
	tax := Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2024, 1, 1), decimal.NewFromInt(200), date(2024, 6, 1))
	assert.True(t, tax.ShortTerm.Equal(decimal.NewFromInt(15)), "expected 15%% STCG, got %s", tax.ShortTerm)
	assert.True(t, tax.ShortTermTaxable.Equal(decimal.NewFromInt(100)))

	// Short term, on/after rate change: 20%
	tax = Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2024, 1, 1), decimal.NewFromInt(200), date(2024, 8, 1))
	assert.True(t, tax.ShortTerm.Equal(decimal.NewFromInt(20)), "expected 20%% STCG, got %s", tax.ShortTerm)

	// Long term, before rate change: 10%
	tax = Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2022, 1, 1), decimal.NewFromInt(200), date(2024, 6, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(10)), "expected 10%% LTCG, got %s", tax.LongTerm)
	assert.True(t, tax.LongTermTaxable.Equal(decimal.NewFromInt(100)))

	// Long term, on/after rate change: 12.5%
	tax = Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2022, 1, 1), decimal.NewFromInt(200), date(2024, 8, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)), "expected 12.5%% LTCG, got %s", tax.LongTerm)

	// Sold exactly on the rate change date: new rate applies
	tax = Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2022, 1, 1), decimal.NewFromInt(200), date(2024, 7, 23))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)))

	// Pre-2018 grandfather clause untouched
	tax = Calculate(db, one, commodity(config.Equity), decimal.NewFromInt(100), date(2016, 1, 1), decimal.NewFromInt(200), date(2017, 1, 1))
	assert.True(t, tax.Taxable.IsZero())
}

func TestDebtRates(t *testing.T) {
	db := testDB(t)
	one := decimal.NewFromInt(1)

	// Purchased on/after 2023-04-01: always slab, regardless of holding period or sell date
	tax := Calculate(db, one, commodity(config.Debt), decimal.NewFromInt(100), date(2023, 5, 1), decimal.NewFromInt(200), date(2027, 1, 1))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(100)))
	assert.True(t, tax.ShortTermTaxable.Equal(decimal.NewFromInt(100)))
	assert.True(t, tax.LongTerm.IsZero())

	// Purchased before 2023-04-01, sold before rate change, held > 3 years: 20% with indexation
	tax = Calculate(db, one, commodity(config.Debt), decimal.NewFromInt(100), date(2020, 6, 1), decimal.NewFromInt(200), date(2024, 1, 1))
	indexedPurchase := decimal.NewFromInt(100).Mul(decimal.NewFromInt(363).Div(decimal.NewFromInt(301)))
	expectedTaxable := decimal.NewFromInt(200).Sub(indexedPurchase)
	assert.True(t, tax.LongTerm.Equal(expectedTaxable.Mul(decimal.NewFromFloat(0.20))), "got %s", tax.LongTerm)
	assert.True(t, tax.LongTermTaxable.Equal(expectedTaxable))

	// Purchased before 2023-04-01, sold on/after rate change, held > 2 years (but < 3 years):
	// new 24-month threshold applies, 12.5% without indexation
	tax = Calculate(db, one, commodity(config.Debt), decimal.NewFromInt(100), date(2022, 6, 1), decimal.NewFromInt(200), date(2024, 12, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)), "got %s", tax.LongTerm)
	assert.True(t, tax.LongTermTaxable.Equal(decimal.NewFromInt(100)))

	// Purchased before 2023-04-01, sold on/after rate change, held < 2 years: short term, slab
	tax = Calculate(db, one, commodity(config.Debt), decimal.NewFromInt(100), date(2023, 1, 1), decimal.NewFromInt(200), date(2024, 8, 1))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(100)))
	assert.True(t, tax.LongTerm.IsZero())
}

func TestEquity35Rates(t *testing.T) {
	db := testDB(t)
	one := decimal.NewFromInt(1)

	// Sold before rate change, held > 3 years: 20%
	tax := Calculate(db, one, commodity(config.Equity35), decimal.NewFromInt(100), date(2020, 6, 1), decimal.NewFromInt(200), date(2024, 1, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromInt(20)), "got %s", tax.LongTerm)

	// Sold on/after rate change, held > 2 years (but < 3 years): 12.5%
	tax = Calculate(db, one, commodity(config.Equity35), decimal.NewFromInt(100), date(2022, 6, 1), decimal.NewFromInt(200), date(2024, 12, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)), "got %s", tax.LongTerm)

	// Sold on/after rate change, held < 2 years: short term, slab
	tax = Calculate(db, one, commodity(config.Equity35), decimal.NewFromInt(100), date(2023, 6, 1), decimal.NewFromInt(200), date(2024, 8, 1))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(100)))
}

func TestUnlistedEquityRates(t *testing.T) {
	db := testDB(t)
	one := decimal.NewFromInt(1)

	// Threshold unchanged (24 months): sold before rate change, held > 2 years: 20% with indexation
	tax := Calculate(db, one, commodity(config.UnlistedEquity), decimal.NewFromInt(100), date(2021, 6, 1), decimal.NewFromInt(200), date(2024, 1, 1))
	indexedPurchase := decimal.NewFromInt(100).Mul(decimal.NewFromInt(363).Div(decimal.NewFromInt(317)))
	expectedTaxable := decimal.NewFromInt(200).Sub(indexedPurchase)
	assert.True(t, tax.LongTerm.Equal(expectedTaxable.Mul(decimal.NewFromFloat(0.20))), "got %s", tax.LongTerm)

	// Sold on/after rate change, held > 2 years: 12.5% without indexation
	tax = Calculate(db, one, commodity(config.UnlistedEquity), decimal.NewFromInt(100), date(2022, 6, 1), decimal.NewFromInt(200), date(2024, 12, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)), "got %s", tax.LongTerm)
	assert.True(t, tax.LongTermTaxable.Equal(decimal.NewFromInt(100)))

	// Held < 2 years: short term, slab, regardless of date
	tax = Calculate(db, one, commodity(config.UnlistedEquity), decimal.NewFromInt(100), date(2023, 6, 1), decimal.NewFromInt(200), date(2024, 8, 1))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(100)))
}

func TestGoldETFRates(t *testing.T) {
	db := testDB(t)
	one := decimal.NewFromInt(1)

	tax := Calculate(db, one, commodity(config.GoldETF), decimal.NewFromInt(100), date(2022, 1, 1), decimal.NewFromInt(200), date(2024, 1, 1))
	assert.True(t, tax.LongTerm.Equal(decimal.NewFromFloat(12.5)))
	assert.True(t, tax.LongTermTaxable.Equal(decimal.NewFromInt(100)))

	tax = Calculate(db, one, commodity(config.GoldETF), decimal.NewFromInt(100), date(2024, 1, 1), decimal.NewFromInt(200), date(2024, 6, 1))
	assert.True(t, tax.Slab.Equal(decimal.NewFromInt(100)))
	assert.True(t, tax.ShortTermTaxable.Equal(decimal.NewFromInt(100)))
}

func TestAdd(t *testing.T) {
	a := Tax{Gain: decimal.NewFromInt(1), Taxable: decimal.NewFromInt(2), Slab: decimal.NewFromInt(3), LongTerm: decimal.NewFromInt(4), ShortTerm: decimal.NewFromInt(5), LongTermTaxable: decimal.NewFromInt(6), ShortTermTaxable: decimal.NewFromInt(7)}
	b := Tax{Gain: decimal.NewFromInt(1), Taxable: decimal.NewFromInt(2), Slab: decimal.NewFromInt(3), LongTerm: decimal.NewFromInt(4), ShortTerm: decimal.NewFromInt(5), LongTermTaxable: decimal.NewFromInt(6), ShortTermTaxable: decimal.NewFromInt(7)}
	sum := Add(a, b)
	assert.True(t, sum.LongTermTaxable.Equal(decimal.NewFromInt(12)))
	assert.True(t, sum.ShortTermTaxable.Equal(decimal.NewFromInt(14)))
}
