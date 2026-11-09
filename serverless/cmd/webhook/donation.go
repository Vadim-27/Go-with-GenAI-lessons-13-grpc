package main

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
)

// Donation — нормалізована пожертва книги.
type Donation struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	ISBN       string `json:"isbn"`
	Condition  string `json:"condition"`
	DonorEmail string `json:"donor_email"`
}

var (
	spaces         = regexp.MustCompile(`\s+`)
	validCondition = map[string]bool{"new": true, "good": true, "fair": true, "poor": true}
)

// normalize перевіряє та нормалізує пожертву. Повертає ВСІ помилки валідації
// одразу, а не лише першу — так клієнту зручніше виправляти запит.
func normalize(in Donation) (Donation, []string) {
	var problems []string
	out := Donation{
		Title:     collapse(in.Title),
		Author:    collapse(in.Author),
		Condition: strings.ToLower(strings.TrimSpace(in.Condition)),
	}

	if out.Title == "" {
		problems = append(problems, "title is required")
	}
	if out.Author == "" {
		problems = append(problems, "author is required")
	}

	isbn, err := normalizeISBN(in.ISBN)
	if err != nil {
		problems = append(problems, "isbn: "+err.Error())
	}
	out.ISBN = isbn

	if !validCondition[out.Condition] {
		problems = append(problems, "condition must be one of: new, good, fair, poor")
	}

	addr, err := mail.ParseAddress(strings.TrimSpace(in.DonorEmail))
	if err != nil {
		problems = append(problems, "donor_email is not a valid email address")
	} else {
		out.DonorEmail = strings.ToLower(addr.Address)
	}

	return out, problems
}

func collapse(s string) string {
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}

// normalizeISBN прибирає дефіси/пробіли та перевіряє контрольну цифру
// (ISBN-10 або ISBN-13).
func normalizeISBN(raw string) (string, error) {
	s := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(raw))
	switch len(s) {
	case 10:
		if !validISBN10(s) {
			return "", fmt.Errorf("invalid ISBN-10 checksum")
		}
	case 13:
		if !validISBN13(s) {
			return "", fmt.Errorf("invalid ISBN-13 checksum")
		}
	default:
		return "", fmt.Errorf("must contain 10 or 13 digits")
	}
	return s, nil
}

func validISBN10(s string) bool {
	sum := 0
	for i, r := range s {
		var v int
		switch {
		case r >= '0' && r <= '9':
			v = int(r - '0')
		case r == 'X' && i == 9:
			v = 10
		default:
			return false
		}
		sum += v * (10 - i)
	}
	return sum%11 == 0
}

func validISBN13(s string) bool {
	sum := 0
	for i, r := range s {
		if r < '0' || r > '9' {
			return false
		}
		w := 1
		if i%2 == 1 {
			w = 3
		}
		sum += int(r-'0') * w
	}
	return sum%10 == 0
}
