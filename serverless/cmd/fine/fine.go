package main

import (
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// LoanReturn — вхідні дані: позика та фактична дата повернення.
type LoanReturn struct {
	LoanID       string `json:"loan_id"`
	Category     string `json:"category"`
	DueDate      string `json:"due_date"`
	ReturnedDate string `json:"returned_date"`
}

// FineResult — результат розрахунку. Гроші — цілі копійки (без float).
type FineResult struct {
	LoanID        string `json:"loan_id"`
	Category      string `json:"category"`
	DaysOverdue   int    `json:"days_overdue"`
	RatePerDay    int    `json:"rate_per_day_kopecks"`
	FineKopecks   int    `json:"fine_kopecks"`
	FineFormatted string `json:"fine"`
	Capped        bool   `json:"capped"`
}

// tariff — ставка за категорією книги: копійок за день та верхня межа штрафу.
type tariff struct{ perDay, cap int }

var tariffs = map[string]tariff{
	"standard":    {perDay: 200, cap: 20000},   // 2 грн/день, максимум 200 грн
	"new_release": {perDay: 500, cap: 50000},   // 5 грн/день, максимум 500 грн
	"reference":   {perDay: 1000, cap: 100000}, // 10 грн/день, максимум 1000 грн
}

// calculate перевіряє вхід і рахує штраф. Повертає ВСІ помилки валідації разом.
// Чиста функція: не читає годинник, тому детермінована й легко тестується.
func calculate(in LoanReturn) (FineResult, []string) {
	var problems []string
	res := FineResult{
		LoanID:   strings.TrimSpace(in.LoanID),
		Category: strings.ToLower(strings.TrimSpace(in.Category)),
	}

	if res.LoanID == "" {
		problems = append(problems, "loan_id is required")
	}
	t, ok := tariffs[res.Category]
	if !ok {
		problems = append(problems, "category must be one of: standard, new_release, reference")
	}

	due, errDue := time.Parse(dateLayout, strings.TrimSpace(in.DueDate))
	if errDue != nil {
		problems = append(problems, "due_date must be a valid date in YYYY-MM-DD format")
	}
	ret, errRet := time.Parse(dateLayout, strings.TrimSpace(in.ReturnedDate))
	if errRet != nil {
		problems = append(problems, "returned_date must be a valid date in YYYY-MM-DD format")
	}
	if len(problems) > 0 {
		return res, problems
	}

	if ret.After(due) {
		res.DaysOverdue = int(ret.Sub(due).Hours() / 24)
	}
	res.RatePerDay = t.perDay
	res.FineKopecks = res.DaysOverdue * t.perDay
	if res.FineKopecks > t.cap {
		res.FineKopecks = t.cap
		res.Capped = true
	}
	res.FineFormatted = fmt.Sprintf("%d.%02d UAH", res.FineKopecks/100, res.FineKopecks%100)
	return res, nil
}
