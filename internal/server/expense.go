package server

import (
	"sort"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Node struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type Link struct {
	Source uint            `json:"source"`
	Target uint            `json:"target"`
	Value  decimal.Decimal `json:"value"`
}

type Pair struct {
	Source uint `json:"source"`
	Target uint `json:"target"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

type ExpenseAggregate struct {
	Account        string          `json:"account"`
	Amount         decimal.Decimal `json:"amount"`
	PreviousAmount decimal.Decimal `json:"previousAmount"`
	ChangePercent  float64         `json:"changePercent"`
}

func GetCurrentExpense(db *gorm.DB) map[string][]posting.Posting {
	expenses := query.Init(db).LastNMonths(3).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").All()
	return utils.GroupByMonth(expenses)
}

func GetExpense(db *gorm.DB) gin.H {
	expenses := query.Init(db).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").All()
	incomes := query.Init(db).Like("Income:%").All()
	investments := query.Init(db).Like("Assets:%").NotAccountPrefix("Assets:Checking").All()
	taxes := query.Init(db).AccountPrefix("Expenses:Tax").All()
	postings := query.Init(db).All()

	graph := make(map[string]Graph)
	for fy, ps := range utils.GroupByFY(postings) {
		graph[fy] = sortGraph(computeHierarchyGraph(ps))
	}

	return gin.H{
		"expenses": expenses,
		"month_wise": gin.H{
			"expenses":    utils.GroupByMonth(expenses),
			"incomes":     utils.GroupByMonth(incomes),
			"investments": utils.GroupByMonth(investments),
			"taxes":       utils.GroupByMonth(taxes)},
		"year_wise": gin.H{
			"expenses":    utils.GroupByMonth(expenses),
			"incomes":     utils.GroupByFY(incomes),
			"investments": utils.GroupByFY(investments),
			"taxes":       utils.GroupByFY(taxes)},
		"graph": graph}
}

func GetExpenseAllocation(db *gorm.DB, c *gin.Context) gin.H {
	// Get month parameter from query string
	month := c.Query("month")

	var expenses []posting.Posting
	var previousExpenses []posting.Posting

	if month != "" {
		// Filter expenses for the specific month
		allExpenses := query.Init(db).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").All()
		expensesByMonth := utils.GroupByMonth(allExpenses)
		if monthExpenses, exists := expensesByMonth[month]; exists {
			expenses = monthExpenses
		} else {
			// No expenses for this month, return empty aggregates
			expenses = []posting.Posting{}
		}

		// Get previous month for comparison
		prevMonth := getPreviousMonth(month)
		if prevMonthExpenses, exists := expensesByMonth[prevMonth]; exists {
			previousExpenses = prevMonthExpenses
		}
	} else {
		// If no month specified, get all expenses
		expenses = query.Init(db).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").All()
	}

	aggregates := computeExpenseAggregateWithChange(expenses, previousExpenses)
	return gin.H{
		"aggregates": aggregates,
	}
}

func getPreviousMonth(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return ""
	}
	return t.AddDate(0, -1, 0).Format("2006-01")
}

func computeExpenseAggregateWithChange(postings []posting.Posting, previousPostings []posting.Posting) map[string]ExpenseAggregate {
	// Group postings by account and sum amounts
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string { return p.Account })
	byAccountPrev := lo.GroupBy(previousPostings, func(p posting.Posting) string { return p.Account })

	result := make(map[string]ExpenseAggregate)

	// Single pass: only process leaf accounts (accounts with actual postings)
	for account, ps := range byAccount {
		// Sum up all amounts for this account
		amount := decimal.Zero
		for _, p := range ps {
			amount = amount.Add(p.Amount)
		}

		// Calculate previous amount
		prevAmount := decimal.Zero
		if prevPs, exists := byAccountPrev[account]; exists {
			for _, p := range prevPs {
				prevAmount = prevAmount.Add(p.Amount)
			}
		}

		// Calculate change percentage
		var changePercent float64
		if !prevAmount.IsZero() {
			change := amount.Sub(prevAmount)
			changePercent, _ = change.Div(prevAmount).Mul(decimal.NewFromInt(100)).Float64()
		}

		// Only add leaf accounts - D3 will calculate parent values automatically
		result[account] = ExpenseAggregate{
			Account:        account,
			Amount:         amount,
			PreviousAmount: prevAmount,
			ChangePercent:  changePercent,
		}
	}

	return result
}

func computeExpenseAggregate(postings []posting.Posting) map[string]ExpenseAggregate {
	// Group postings by account and sum amounts
	byAccount := lo.GroupBy(postings, func(p posting.Posting) string { return p.Account })
	result := make(map[string]ExpenseAggregate)

	// Single pass: only process leaf accounts (accounts with actual postings)
	for account, ps := range byAccount {
		// Sum up all amounts for this account
		amount := decimal.Zero
		for _, p := range ps {
			amount = amount.Add(p.Amount)
		}

		// Only add leaf accounts - D3 will calculate parent values automatically
		result[account] = ExpenseAggregate{Account: account, Amount: amount}
	}

	return result
}

type ExpenseTransaction struct {
	Date      time.Time       `json:"date"`
	Payee     string          `json:"payee"`
	Account   string          `json:"account"`
	Amount    decimal.Decimal `json:"amount"`
	FileName  string          `json:"filename"`
	BeginLine uint64          `json:"beginLine"`
	EndLine   uint64          `json:"endLine"`
}

func GetExpenseTransactions(db *gorm.DB, c *gin.Context) gin.H {
	account := c.Query("account")
	month := c.Query("month")
	limitStr := c.DefaultQuery("limit", "50")

	limit := 50
	if l, err := time.ParseDuration(limitStr + "s"); err == nil {
		limit = int(l.Seconds())
	}

	// Get all expenses
	allExpenses := query.Init(db).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").Desc().All()

	// Filter by account prefix
	if account != "" {
		allExpenses = lo.Filter(allExpenses, func(p posting.Posting, _ int) bool {
			return strings.HasPrefix(p.Account, account)
		})
	}

	// Filter by month if specified
	if month != "" {
		allExpenses = lo.Filter(allExpenses, func(p posting.Posting, _ int) bool {
			return p.Date.Format("2006-01") == month
		})
	}

	// Limit results
	if len(allExpenses) > limit {
		allExpenses = allExpenses[:limit]
	}

	// Convert to transaction format
	transactions := lo.Map(allExpenses, func(p posting.Posting, _ int) ExpenseTransaction {
		return ExpenseTransaction{
			Date:      p.Date,
			Payee:     p.Payee,
			Account:   p.Account,
			Amount:    p.Amount,
			FileName:  p.FileName,
			BeginLine: p.TransactionBeginLine,
			EndLine:   p.TransactionEndLine,
		}
	})

	return gin.H{
		"transactions": transactions,
		"total":        len(allExpenses),
	}
}

type ExpenseTimelineData struct {
	Month  string          `json:"month"`
	Amount decimal.Decimal `json:"amount"`
}

func GetExpenseTimeline(db *gorm.DB, c *gin.Context) gin.H {
	period := c.Query("period")
	account := c.Query("account") // Optional: specific account to filter by

	// Default to 1 year if no period specified
	if period == "" {
		period = "1y"
	}

	// Calculate start date based on period
	var startDate time.Time
	now := time.Now()
	switch period {
	case "3m":
		startDate = now.AddDate(0, -3, 0)
	case "6m":
		startDate = now.AddDate(0, -6, 0)
	case "1y":
		startDate = now.AddDate(-1, 0, 0)
	case "3y":
		startDate = now.AddDate(-3, 0, 0)
	default:
		startDate = now.AddDate(-1, 0, 0) // Default to 1 year
	}

	// Get all expenses
	allExpenses := query.Init(db).Like("Expenses:%").NotAccountPrefix("Expenses:Tax").All()

	// Filter by account if specified
	if account != "" {
		allExpenses = lo.Filter(allExpenses, func(p posting.Posting, _ int) bool {
			return strings.HasPrefix(p.Account, account)
		})
	}

	// Group by month
	expensesByMonth := utils.GroupByMonth(allExpenses)

	// Generate timeline data for the specified period
	var timelineData []ExpenseTimelineData
	current := startDate

	for current.Before(now) || current.Equal(now.Truncate(24*time.Hour)) {
		monthKey := current.Format("2006-01")
		monthExpenses := expensesByMonth[monthKey]

		totalAmount := decimal.Zero
		for _, expense := range monthExpenses {
			totalAmount = totalAmount.Add(expense.Amount)
		}

		timelineData = append(timelineData, ExpenseTimelineData{
			Month:  monthKey,
			Amount: totalAmount,
		})

		// Move to next month
		current = current.AddDate(0, 1, 0)
	}

	return gin.H{"timeline": timelineData}
}

func sortGraph(graph Graph) Graph {
	nodes := graph.Nodes
	sort.Slice(nodes, func(i, j int) bool {
		return graph.Nodes[i].Name < graph.Nodes[j].Name
	})

	links := graph.Links
	sort.Slice(links, func(i, j int) bool {
		return graph.Links[i].Source < graph.Links[j].Source || (graph.Links[i].Source == graph.Links[j].Source && graph.Links[i].Target < graph.Links[j].Target)
	})
	return Graph{
		Nodes: nodes,
		Links: links,
	}

}

func computeHierarchyGraph(postings []posting.Posting) Graph {
	nodes := make(map[string]Node)
	links := make(map[Pair]decimal.Decimal)

	var nodeID uint = 0

	transactions := transaction.Build(postings)

	for _, p := range postings {
		addNode(&nodeID, &nodes, p.Account)
	}

	for _, t := range transactions {
		from := lo.Filter(t.Postings, func(p posting.Posting, _ int) bool { return p.Amount.LessThan(decimal.Zero) })
		to := lo.Filter(t.Postings, func(p posting.Posting, _ int) bool { return p.Amount.GreaterThan(decimal.Zero) })

		for _, f := range from {
			for f.Amount.Abs().GreaterThan(decimal.NewFromFloat(0.1)) && len(to) > 0 {
				top := to[0]
				if top.Amount.GreaterThan(f.Amount.Neg()) {
					addLink(f.Account, top.Account, f.Amount.Neg(), &nodes, &links)
					top.Amount = top.Amount.Sub(f.Amount)
					f.Amount = decimal.Zero
				} else {
					addLink(f.Account, top.Account, top.Amount, &nodes, &links)
					f.Amount = f.Amount.Add(top.Amount)
					to = to[1:]
				}
			}
		}
	}

	return Graph{Nodes: lo.Values(nodes), Links: lo.Map(lo.Keys(links), func(k Pair, _ int) Link {
		return Link{Source: k.Source, Target: k.Target, Value: links[k]}
	})}

}

func addNode(nodeID *uint, nodes *map[string]Node, account string) {
	if account == "" {
		return
	}

	_, ok := (*nodes)[account]
	if !ok {
		if strings.HasPrefix(account, "Income:") || strings.HasPrefix(account, "Expenses:") {
			parts := strings.Split(account, ":")
			addNode(nodeID, nodes, strings.Join(parts[:len(parts)-1], ":"))

		}

		(*nodeID)++
		(*nodes)[account] = Node{ID: *nodeID, Name: account}
	}
}

func addLink(source string, target string, amount decimal.Decimal, nodes *map[string]Node, links *map[Pair]decimal.Decimal) {

	sparts := strings.Split(source, ":")
	if sparts[0] == "Income" {
		for len(sparts) > 1 {
			s := strings.Join(sparts, ":")
			t := strings.Join(sparts[:len(sparts)-1], ":")
			(*links)[Pair{Source: (*nodes)[s].ID, Target: (*nodes)[t].ID}] = (*links)[Pair{Source: (*nodes)[s].ID, Target: (*nodes)[t].ID}].Add(amount)
			sparts = sparts[:len(sparts)-1]
		}
		source = strings.Join(sparts, ":")

	}

	tparts := strings.Split(target, ":")
	if tparts[0] == "Expenses" {
		for len(tparts) > 1 {
			t := strings.Join(tparts, ":")
			s := strings.Join(tparts[:len(tparts)-1], ":")
			(*links)[Pair{Source: (*nodes)[s].ID, Target: (*nodes)[t].ID}] = (*links)[Pair{Source: (*nodes)[s].ID, Target: (*nodes)[t].ID}].Add(amount)
			tparts = tparts[:len(tparts)-1]
		}
		target = strings.Join(tparts, ":")
	}

	(*links)[Pair{Source: (*nodes)[source].ID, Target: (*nodes)[target].ID}] = (*links)[Pair{Source: (*nodes)[source].ID, Target: (*nodes)[target].ID}].Add(amount)
}
