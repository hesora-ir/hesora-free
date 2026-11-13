package domain

import (
	"fmt"
	"strings"
)

var faOnes = []string{"صفر", "یک", "دو", "سه", "چهار", "پنج", "شش", "هفت", "هشت", "نه", "ده", "یازده", "دوازده", "سیزده", "چهارده", "پانزده", "شانزده", "هفده", "هجده", "نوزده"}
var faTens = []string{"", "", "بیست", "سی", "چهل", "پنجاه", "شصت", "هفتاد", "هشتاد", "نود"}
var faHundreds = []string{"", "یکصد", "دویست", "سیصد", "چهارصد", "پانصد", "ششصد", "هفتصد", "هشتصد", "نهصد"}
var faGroups = []string{"", "هزار", "میلیون", "میلیارد", "تریلیون", "کوادریلیون"}

func AmountInWordsFA(amount Money, currency Currency) string {
	if amount < 0 {
		return "منفی " + AmountInWordsFA(-amount, currency)
	}
	n := int64(amount)
	if n == 0 {
		return "صفر " + currencyFA(currency)
	}
	parts := []string{}
	group := 0
	for n > 0 && group < len(faGroups) {
		chunk := int(n % 1000)
		if chunk > 0 {
			word := faTriplet(chunk)
			if faGroups[group] != "" {
				word += " " + faGroups[group]
			}
			parts = append([]string{word}, parts...)
		}
		n /= 1000
		group++
	}
	if n > 0 {
		return fmt.Sprintf("%d %s", amount, currencyFA(currency))
	}
	return strings.Join(parts, " و ") + " " + currencyFA(currency)
}

func currencyFA(c Currency) string {
	if c == CurrencyRial {
		return "ریال"
	}
	return "تومان"
}
func faTriplet(n int) string {
	parts := []string{}
	if n >= 100 {
		parts = append(parts, faHundreds[n/100])
		n %= 100
	}
	if n >= 20 {
		parts = append(parts, faTens[n/10])
		n %= 10
	}
	if n > 0 {
		parts = append(parts, faOnes[n])
	}
	return strings.Join(parts, " و ")
}
