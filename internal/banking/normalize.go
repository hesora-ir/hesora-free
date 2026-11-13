// Package banking provides offline structural validation for Iranian banking identifiers.
// A valid checksum does not prove that an account exists or belongs to the user.
package banking

import (
	"fmt"
	"strings"
	"unicode"
)

type Bank struct{ Code, Name string }
type Result struct {
	Valid      bool
	Normalized string
	Bank       *Bank
	Code       string
}

var bankCodes = map[string]Bank{
	"010": {"010", "بانک مرکزی"}, "011": {"011", "بانک صنعت و معدن"}, "012": {"012", "بانک ملت"},
	"013": {"013", "بانک رفاه کارگران"}, "014": {"014", "بانک مسکن"}, "015": {"015", "بانک سپه"},
	"016": {"016", "بانک کشاورزی"}, "017": {"017", "بانک ملی ایران"}, "018": {"018", "بانک تجارت"},
	"019": {"019", "بانک صادرات ایران"}, "020": {"020", "بانک توسعه صادرات"}, "021": {"021", "پست بانک ایران"},
	"022": {"022", "بانک توسعه تعاون"}, "051": {"051", "مؤسسه اعتباری توسعه"}, "052": {"052", "بانک قوامین"},
	"053": {"053", "بانک کارآفرین"}, "054": {"054", "بانک پارسیان"}, "055": {"055", "بانک اقتصاد نوین"},
	"056": {"056", "بانک سامان"}, "057": {"057", "بانک پاسارگاد"}, "058": {"058", "بانک سرمایه"},
	"059": {"059", "بانک سینا"}, "060": {"060", "بانک قرض‌الحسنه مهر ایران"}, "061": {"061", "بانک شهر"},
	"062": {"062", "بانک آینده"}, "063": {"063", "بانک انصار"}, "064": {"064", "بانک گردشگری"},
	"065": {"065", "بانک حکمت ایرانیان"}, "066": {"066", "بانک دی"}, "069": {"069", "بانک ایران‌زمین"},
	"070": {"070", "بانک رسالت"}, "073": {"073", "بانک مهر اقتصاد"}, "075": {"075", "بانک ملی"},
}

var cardPrefixes = map[string]Bank{
	"603799": bankCodes["017"], "589210": bankCodes["015"], "627381": bankCodes["015"],
	"610433": bankCodes["012"], "991975": bankCodes["012"], "627353": bankCodes["018"],
	"627412": bankCodes["055"], "622106": bankCodes["054"], "62198618": {"blu", "بلوبانک"}, "62198619": {"blu", "بلوبانک"},
	"621986": bankCodes["056"], "639347": bankCodes["057"], "628023": bankCodes["014"],
	"627648": bankCodes["020"], "639599": bankCodes["060"], "502229": bankCodes["060"],
	"502908": bankCodes["061"], "505809": bankCodes["057"], "627488": bankCodes["058"],
	"639346": bankCodes["059"], "505416": bankCodes["064"], "505785": bankCodes["062"],
	"636214": bankCodes["069"], "505701": bankCodes["070"], "639370": bankCodes["018"],
}

// NormalizeBankNumber converts Persian/Arabic digits to ASCII and removes common separators.
func NormalizeBankNumber(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= '۰' && r <= '۹':
			r = '0' + (r - '۰')
		case r >= '٠' && r <= '٩':
			r = '0' + (r - '٠')
		case unicode.IsSpace(r) || r == '-' || r == '‐' || r == '‑' || r == '\u200e' || r == '\u200f' || r == '\u061c' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069'):
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func DetectBankByCard(value string) *Bank {
	n := NormalizeBankNumber(value)
	for size := 8; size >= 6; size-- {
		if len(n) >= size {
			if b, ok := cardPrefixes[n[:size]]; ok {
				return &b
			}
		}
	}
	return nil
}

func ValidateCard(value string) Result {
	n := NormalizeBankNumber(value)
	out := Result{Normalized: n, Bank: DetectBankByCard(value)}
	if len(n) != 16 {
		out.Code = "INVALID_LENGTH"
		return out
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			out.Code = "INVALID_CHARACTERS"
			return out
		}
	}
	if strings.Count(n, n[:1]) == len(n) {
		out.Code = "INVALID_CHECKSUM"
		return out
	}
	sum := 0
	for i := 0; i < len(n); i++ {
		d := int(n[len(n)-1-i] - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	if sum%10 != 0 {
		out.Code = "INVALID_CHECKSUM"
		return out
	}
	out.Valid = true
	if out.Bank == nil {
		out.Code = "UNKNOWN_BANK"
	}
	return out
}

func ValidateIBAN(value string) Result {
	n := strings.ToUpper(NormalizeBankNumber(value))
	// Users often paste the 24 digits from a bank app without the IR prefix.
	if len(n) == 24 && n[0] >= '0' && n[0] <= '9' {
		n = "IR" + n
	}
	out := Result{Normalized: n}
	if len(n) != 26 {
		out.Code = "INVALID_LENGTH"
		return out
	}
	if !strings.HasPrefix(n, "IR") {
		out.Code = "INVALID_IBAN_PREFIX"
		return out
	}
	for _, r := range n[2:] {
		if r < '0' || r > '9' {
			out.Code = "INVALID_CHARACTERS"
			return out
		}
	}
	if n[2:4] < "02" || n[2:4] > "98" {
		out.Code = "INVALID_CHECKSUM"
		return out
	}
	code := n[4:7]
	if b, ok := bankCodes[code]; ok {
		out.Bank = &b
	} else {
		out.Code = "INVALID_IBAN_BANK_CODE"
		return out
	}
	rotated := n[4:] + n[:4]
	mod := 0
	for _, r := range rotated {
		var digits string
		if r >= '0' && r <= '9' {
			digits = string(r)
		} else {
			digits = fmt.Sprintf("%d", r-'A'+10)
		}
		for _, d := range digits {
			mod = (mod*10 + int(d-'0')) % 97
		}
	}
	if mod != 1 {
		out.Code = "INVALID_CHECKSUM"
		return out
	}
	out.Valid = true
	return out
}

// ResolveBankName preserves the recorded name of historical invoices, while
// filling missing names in older snapshots from a valid IBAN or card number.
func ResolveBankName(name, iban, card string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	if result := ValidateIBAN(iban); result.Valid && result.Bank != nil {
		return result.Bank.Name
	}
	if result := ValidateCard(card); result.Valid && result.Bank != nil {
		return result.Bank.Name
	}
	return ""
}

// DisplayIBAN repairs digit direction and separators in legacy snapshots without
// silently changing invalid historical data or recalculating a check digit.
func DisplayIBAN(value string) string {
	if result := ValidateIBAN(value); result.Valid {
		return result.Normalized
	}
	return value
}

// ValidateAccount checks only normalized numeric structure; bank-specific account rules vary.
func ValidateAccount(value string) Result {
	n := NormalizeBankNumber(value)
	out := Result{Normalized: n}
	if len(n) < 4 || len(n) > 30 {
		out.Code = "INVALID_LENGTH"
		return out
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			out.Code = "INVALID_CHARACTERS"
			return out
		}
	}
	out.Valid = true
	return out
}

// ConvertAccountToIBAN intentionally reports unsupported: account-to-IBAN algorithms are bank/account-type specific.
func ConvertAccountToIBAN(account, bankCode string) (string, error) {
	if _, ok := bankCodes[bankCode]; !ok {
		return "", fmt.Errorf("UNKNOWN_BANK")
	}
	return "", fmt.Errorf("CONVERSION_UNAVAILABLE")
}
