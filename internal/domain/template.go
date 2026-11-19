package domain

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"strings"
	"time"
)

var safeCSSColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var safeFontName = regexp.MustCompile(`^[A-Za-z0-9 _-]{1,80}$`)

type Letterhead struct {
	ID              int64
	IssuerProfileID int64
	Name            string
	LogoPath        string
	Title           string
	Subtitle        string
	ContactLine     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (h Letterhead) Validate() error {
	if strings.TrimSpace(h.Name) == "" || len(h.Name) > 200 {
		return errors.New("نام سربرگ الزامی است و حداکثر ۲۰۰ نویسه دارد")
	}
	if h.IssuerProfileID < 0 {
		return errors.New("شناسه صادرکننده نامعتبر است")
	}
	if len(h.LogoPath) > 2048 || len(h.Title) > 300 || len(h.Subtitle) > 500 || len(h.ContactLine) > 1000 {
		return errors.New("اطلاعات سربرگ بیش از حد مجاز است")
	}
	return nil
}

type BankAccount struct {
	ID              int64
	IssuerProfileID int64
	BankName        string
	AccountHolder   string
	AccountNumber   string
	CardNumber      string
	IBAN            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (a BankAccount) Validate() error {
	if a.IssuerProfileID < 0 {
		return errors.New("شناسه صادرکننده نامعتبر است")
	}
	if strings.TrimSpace(a.BankName) == "" || len(a.BankName) > 200 {
		return errors.New("نام بانک الزامی است")
	}
	if strings.TrimSpace(a.AccountHolder) == "" || len(a.AccountHolder) > 200 {
		return errors.New("نام صاحب حساب الزامی است")
	}
	if len(a.AccountNumber) > 100 || len(a.CardNumber) > 100 || len(a.IBAN) > 100 {
		return errors.New("اطلاعات حساب بانکی بیش از حد مجاز است")
	}
	return nil
}

type DocumentTemplate struct {
	ID               int64
	Name             string
	Prefix           string
	ThankYou         string
	LogoData         string
	SealData         string
	SignatureData    string
	PageSize         string
	Orientation      string
	FontFamily       string
	FontSize         int
	PrimaryColor     string
	AccentColor      string
	LetterheadID     int64
	BankAccountIDs   []int64
	ShowLogo         bool
	ShowTax          bool
	ShowBankAccounts bool
	ShowSeal         bool
	ShowSignature    bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (t DocumentTemplate) Validate() error {
	if strings.TrimSpace(t.Name) == "" || len(t.Name) > 200 {
		return errors.New("نام قالب الزامی است")
	}
	if len(t.Prefix) > 100 || len(t.ThankYou) > 2000 {
		return errors.New("پیشوند یا متن پایانی قالب بیش از حد مجاز است")
	}
	for _, asset := range []string{t.LogoData, t.SealData, t.SignatureData} {
		if err := validateTemplateImage(asset); err != nil {
			return err
		}
	}
	if t.PageSize != "A4" && t.PageSize != "Letter" {
		return errors.New("اندازه صفحه باید A4 یا Letter باشد")
	}
	if t.Orientation != "portrait" && t.Orientation != "landscape" {
		return errors.New("جهت صفحه نامعتبر است")
	}
	if !safeFontName.MatchString(t.FontFamily) {
		return errors.New("نام فونت نامعتبر است")
	}
	if t.FontSize < 8 || t.FontSize > 32 {
		return errors.New("اندازه فونت باید بین ۸ تا ۳۲ باشد")
	}
	if !safeCSSColor.MatchString(t.PrimaryColor) || !safeCSSColor.MatchString(t.AccentColor) {
		return errors.New("رنگ قالب باید به صورت #RRGGBB باشد")
	}
	if t.LetterheadID < 0 {
		return errors.New("شناسه سربرگ نامعتبر است")
	}
	seen := make(map[int64]struct{}, len(t.BankAccountIDs))
	for _, id := range t.BankAccountIDs {
		if id <= 0 {
			return errors.New("شناسه حساب بانکی نامعتبر است")
		}
		if _, ok := seen[id]; ok {
			return errors.New("حساب بانکی تکراری است")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateTemplateImage(asset string) error {
	if asset == "" {
		return nil
	}
	const maxEncoded = 700_000
	if len(asset) > maxEncoded || !strings.HasPrefix(asset, "data:image/") {
		return errors.New("تصویر قالب باید PNG یا JPEG و حداکثر ۵۰۰ کیلوبایت باشد")
	}
	var mime, encoded string
	if strings.HasPrefix(asset, "data:image/png;base64,") {
		mime, encoded = "image/png", strings.TrimPrefix(asset, "data:image/png;base64,")
	} else if strings.HasPrefix(asset, "data:image/jpeg;base64,") {
		mime, encoded = "image/jpeg", strings.TrimPrefix(asset, "data:image/jpeg;base64,")
	} else {
		return errors.New("قالب فقط از تصویر PNG یا JPEG پشتیبانی می‌کند")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > 512_000 {
		return errors.New("اندازه یا کدگذاری تصویر قالب نامعتبر است")
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || decoded == nil || decoded.Bounds().Dx() < 1 || decoded.Bounds().Dy() < 1 || decoded.Bounds().Dx() > 3000 || decoded.Bounds().Dy() > 3000 ||
		(mime == "image/png" && format != "png") || (mime == "image/jpeg" && format != "jpeg") {
		return errors.New("تصویر قالب خراب است یا ابعاد آن از ۳۰۰۰ در ۳۰۰۰ بیشتر است")
	}
	return nil
}

func EncodeTemplateImage(data []byte, mime string) (string, error) {
	if mime != "image/png" && mime != "image/jpeg" {
		return "", errors.New("تصویر قالب فقط می‌تواند PNG یا JPEG باشد")
	}
	asset := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	if err := validateTemplateImage(asset); err != nil {
		return "", err
	}
	return asset, nil
}
