package licensing

import "errors"

const (
	FreeCustomerLimit      = 50
	FreeInvoiceLimit       = 200
	FreeIssuerProfileLimit = 1
	ProFullFeature         = "pro.full"
)

var ErrFeatureNotLicensed = errors.New("این قابلیت در مجوز فعلی فعال نیست")

type Feature struct {
	ID             string
	Name           string
	IncludedInFree bool
}

var catalog = []Feature{
	{ID: "basic.invoice", Name: "ایجاد و مشاهده فاکتور پایه", IncludedInFree: true},
	{ID: "basic.customers", Name: "مدیریت مشتریان", IncludedInFree: true},
	{ID: ProFullFeature, Name: "نسخه پرو Hesora"},
	{ID: "export.pdf", Name: "خروجی PDF پیشرفته"},
	{ID: "export.excel", Name: "خروجی Excel پیشرفته"},
	{ID: "templates.custom", Name: "قالب‌ها و سربرگ‌های سفارشی"},
	{ID: "payments.advanced", Name: "تخصیص پیشرفته پرداخت‌ها"},
	{ID: "backup.advanced", Name: "پشتیبان‌گیری پیشرفته"},
}

func FeatureCatalog() []Feature {
	out := make([]Feature, len(catalog))
	copy(out, catalog)
	return out
}

func DefaultFreeFeatures() []string {
	out := make([]string, 0, len(catalog))
	for _, feature := range catalog {
		if feature.IncludedInFree {
			out = append(out, feature.ID)
		}
	}
	return out
}

func RequireFeature(result Result, feature string) error {
	if result.HasFeature(feature) {
		return nil
	}
	return ErrFeatureNotLicensed
}
