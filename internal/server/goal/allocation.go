package goal

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
)

type GoalAllocationTarget struct {
	Name          string                 `json:"name"`
	Target        decimal.Decimal        `json:"target"`
	Current       decimal.Decimal        `json:"current"`
	CurrentAmount decimal.Decimal        `json:"current_amount"`
	TargetAmount  decimal.Decimal        `json:"target_amount"`
	Accounts      []string               `json:"accounts"`
	Children      []GoalAllocationTarget `json:"children"`
}

func computeGoalAllocationTargets(postings []posting.Posting, targets []config.AllocationTarget) []GoalAllocationTarget {
	if len(targets) == 0 || len(postings) == 0 {
		return nil
	}
	total := accounting.CurrentBalance(postings)
	if total.IsZero() {
		return nil
	}
	var result []GoalAllocationTarget
	for _, t := range targets {
		result = append(result, computeGoalAllocationTarget(postings, t, total))
	}
	return result
}

func computeGoalAllocationTarget(postings []posting.Posting, t config.AllocationTarget, total decimal.Decimal) GoalAllocationTarget {
	filtered := accounting.FilterByGlob(postings, t.Accounts)
	currentTotal := accounting.CurrentBalance(filtered)
	current := decimal.Zero
	if !total.IsZero() {
		current = currentTotal.Div(total).Mul(decimal.NewFromInt(100))
	}
	var children []GoalAllocationTarget
	if len(t.Children) > 0 && !currentTotal.IsZero() {
		for _, child := range t.Children {
			children = append(children, computeGoalAllocationTarget(filtered, child, currentTotal))
		}
	}
	targetAmount := total.Mul(decimal.NewFromFloat(t.Target)).Div(decimal.NewFromInt(100))
	return GoalAllocationTarget{
		Name:          t.Name,
		Target:        decimal.NewFromFloat(t.Target),
		Current:       current,
		CurrentAmount: currentTotal,
		TargetAmount:  targetAmount,
		Accounts:      t.Accounts,
		Children:      children,
	}
}
