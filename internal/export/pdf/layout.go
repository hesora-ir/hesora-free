package pdf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"
	"github.com/hesora-ir/hesora-free/internal/domain"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
)

var ink = [3]int{29, 41, 57}
var muted = [3]int{102, 112, 133}
var rule = [3]int{216, 225, 235}
var pale = [3]int{238, 243, 248}
var white = [3]int{255, 255, 255}

type invoiceLayout struct {
	p                                            *fpdf.Fpdf
	data                                         invoicehtml.PreviewData
	left, right, width, bottom, size, lineHeight float64
	primary, accent                              [3]int
}

func renderInvoice(data invoicehtml.PreviewData, customer, issuer invoicehtml.PartySnapshot, banks []domain.BankAccount, output string, opts Options) error {
	orientation := "P"
	if data.Template.Orientation == "landscape" {
		orientation = "L"
	}
	p := fpdf.New(orientation, "mm", data.Template.PageSize, "")
	p.SetMargins(18, 18, 18)
	p.SetAutoPageBreak(false, 18)
	p.SetFontLocation(opts.FontDirectory)
	p.AddUTF8Font("InvoiceFont", "", opts.FontName)
	bold := strings.TrimSuffix(opts.FontName, "-Regular.ttf") + "-Bold.ttf"
	if _, err := os.Stat(filepath.Join(opts.FontDirectory, bold)); err != nil {
		bold = opts.FontName
	}
	p.AddUTF8Font("InvoiceFont", "B", bold)
	if p.Error() != nil {
		return fmt.Errorf("بارگذاری فونت فارسی: %w", p.Error())
	}
	p.SetTitle(data.Document.Number, false)
	p.SetSubject("Invoice", false)
	width, height := p.GetPageSize()
	l := &invoiceLayout{p: p, data: data, left: 18, right: width - 18, width: width - 36, bottom: height - 20, size: float64(data.Template.FontSize), primary: parseColor(data.Template.PrimaryColor, [3]int{40, 80, 130}), accent: parseColor(data.Template.AccentColor, [3]int{71, 119, 168})}
	l.lineHeight = math.Max(5, l.size*0.52)
	p.SetFooterFunc(func() {
		p.SetDrawColor(rule[0], rule[1], rule[2])
		p.Line(l.left, height-15, l.right, height-15)
		l.text(l.left, height-13, l.width/3, 5, faDigits(fmt.Sprintf("صفحه %d", p.PageNo())), 8, "", "L", muted)
		l.text(l.left+l.width/3, height-13, l.width/3, 5, "از اعتماد شما سپاسگزاریم", 8, "", "C", muted)
		l.text(l.left+2*l.width/3, height-13, l.width/3, 5, data.Document.Number, 8, "", "R", muted)
	})
	p.AddPage()
	l.header(issuer)
	l.parties(issuer, customer)
	l.items()
	l.totals()
	if strings.TrimSpace(data.Document.Note) != "" {
		l.section("توضیحات و شرایط", []string{data.Document.Note})
	}
	if data.Template.ShowBankAccounts && len(banks) > 0 {
		var lines []string
		for _, bank := range banks {
			var parts []string
			for _, field := range [][2]string{{"صاحب حساب", bank.AccountHolder}, {"شماره حساب", bank.AccountNumber}, {"کارت", bank.CardNumber}, {"شبا", bank.IBAN}, {"بانک", bank.BankName}} {
				if strings.TrimSpace(field[1]) != "" {
					parts = append(parts, field[0]+":\u00a0"+field[1])
				}
			}
			if len(parts) > 0 {
				lines = append(lines, strings.Join(parts, "  |  "))
			}
		}
		if len(lines) > 0 {
			l.section("اطلاعات پرداخت", lines)
		}
	}
	if strings.TrimSpace(data.Template.ThankYou) != "" {
		l.section("پیام فروشگاه", []string{data.Template.ThankYou})
	}
	if data.Template.ShowSeal || data.Template.ShowSignature {
		l.signatureMarks()
	}
	if err := os.MkdirAll(parent(output), 0755); err != nil {
		return err
	}
	if err := p.OutputFileAndClose(output); err != nil {
		return fmt.Errorf("تولید PDF ناموفق بود: %w", err)
	}
	return nil
}

func (l *invoiceLayout) signatureMarks() {
	labels := make([]struct{ label, data string }, 0, 2)
	if l.data.Template.ShowSeal {
		labels = append(labels, struct{ label, data string }{"مهر فروشگاه", l.data.Template.SealData})
	}
	if l.data.Template.ShowSignature {
		labels = append(labels, struct{ label, data string }{"امضای فروشنده", l.data.Template.SignatureData})
	}
	if len(labels) == 0 {
		return
	}
	const height = 28.0
	l.ensure(height + 5)
	y := l.p.GetY() + 4
	width := l.width / float64(len(labels))
	for i, asset := range labels {
		x := l.left + float64(i)*width
		l.p.SetDrawColor(rule[0], rule[1], rule[2])
		l.p.SetDashPattern([]float64{1.2, 1.2}, 0)
		l.p.RoundedRect(x+2, y, width-4, height, 2, "1234", "D")
		l.p.SetDashPattern([]float64{}, 0)
		if imageName, imageType, ok := registerTemplateImage(l.p, asset.data, fmt.Sprintf("template-mark-%d", i)); ok {
			l.p.ImageOptions(imageName, x+5, y+3, width-10, height-11, true, fpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
		}
		l.text(x+4, y+height-7, width-8, 5, asset.label, 8, "", "C", muted)
	}
	l.p.SetY(y + height + 4)
}

func registerTemplateImage(p *fpdf.Fpdf, data, name string) (string, string, bool) {
	parts := strings.SplitN(data, ",", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	imageType := ""
	switch parts[0] {
	case "data:image/png;base64":
		imageType = "PNG"
	case "data:image/jpeg;base64":
		imageType = "JPG"
	default:
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil || len(decoded) == 0 {
		return "", "", false
	}
	info := p.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, bytes.NewReader(decoded))
	return name, imageType, info != nil && p.Error() == nil
}

// Every string is shaped only after wrapping its logical text into individual
// lines. Reordering a whole paragraph first reverses Persian line order.
func (l *invoiceLayout) text(x, y, w, h float64, value string, size float64, style, align string, color [3]int) {
	l.p.SetFont("InvoiceFont", style, size)
	visual := fa(value)
	if measured := l.p.GetStringWidth(visual); measured > w && measured > 0 {
		l.p.SetFont("InvoiceFont", style, size*(w-0.5)/measured)
	}
	l.p.SetTextColor(color[0], color[1], color[2])
	l.p.SetXY(x, y)
	l.p.CellFormat(w, h, visual, "", 0, align, false, 0, "")
}

func (l *invoiceLayout) box(x, y, w, h float64, fill [3]int) {
	l.p.SetFillColor(fill[0], fill[1], fill[2])
	l.p.SetDrawColor(rule[0], rule[1], rule[2])
	l.p.Rect(x, y, w, h, "DF")
}

func (l *invoiceLayout) wrap(value string, width, size float64, style string) []string {
	l.p.SetFont("InvoiceFont", style, size)
	return wrapLogicalText(value, width, func(text string) float64 { return l.p.GetStringWidth(fa(text)) })
}

func wrapLogicalText(value string, width float64, measure func(string) float64) []string {
	var result []string
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	for _, paragraph := range strings.Split(value, "\n") {
		runes := []rune(strings.TrimSpace(paragraph))
		if len(runes) == 0 {
			result = append(result, "")
			continue
		}
		for len(runes) > 0 {
			end, lastSpace := 0, -1
			for end < len(runes) {
				if end > 0 && measure(string(runes[:end+1])) > width {
					break
				}
				if unicode.IsSpace(runes[end]) && runes[end] != '\u00a0' {
					lastSpace = end
				}
				end++
			}
			if end < len(runes) && lastSpace > 0 {
				end = lastSpace
			}
			if end == 0 {
				end = 1
			}
			result = append(result, strings.TrimSpace(string(runes[:end])))
			runes = []rune(strings.TrimLeftFunc(string(runes[end:]), unicode.IsSpace))
		}
	}
	return result
}

func (l *invoiceLayout) nextPage() {
	l.p.AddPage()
	l.text(l.left, 17, l.width/2, 7, l.data.Document.Number, l.size-1, "", "L", muted)
	l.text(l.left+l.width/2, 17, l.width/2, 7, domain.DocumentTypeLabelFA(l.data.Document)+" • ادامه", l.size, "B", "R", l.primary)
	l.p.SetDrawColor(rule[0], rule[1], rule[2])
	l.p.Line(l.left, 26, l.right, 26)
	l.p.SetY(31)
}

func (l *invoiceLayout) ensure(height float64) {
	if l.p.GetY()+height > l.bottom {
		l.nextPage()
	}
}

func (l *invoiceLayout) header(issuer invoicehtml.PartySnapshot) {
	p := l.p
	brandW, codeW := l.width*0.43, l.width*0.2
	titleW := l.width - brandW - codeW - 6
	logoType := ""
	logoPath := l.data.Letterhead.LogoPath
	if l.data.Template.ShowLogo && l.data.Template.LogoData != "" {
		if imageName, imageType, ok := registerTemplateImage(p, l.data.Template.LogoData, "template-logo"); ok {
			logoPath, logoType = imageName, imageType
		}
	} else if l.data.Template.ShowLogo && logoPath != "" {
		ext := strings.ToLower(filepath.Ext(logoPath))
		if ext == ".png" {
			logoType = "PNG"
		} else if ext == ".jpg" || ext == ".jpeg" {
			logoType = "JPG"
		}
		if _, err := os.Stat(logoPath); err != nil {
			logoType = ""
		}
	}
	brandTextW := brandW - 6
	if logoType != "" {
		brandTextW -= 24
	}
	brand := l.wrap(first(l.data.Letterhead.Title, "فاکتور"), brandTextW, l.size+2, "B")
	subtitle := l.wrap(first(l.data.Letterhead.Subtitle, l.data.Letterhead.ContactLine, "فروش و خدمات حرفه ای"), brandTextW, l.size-1, "")
	code := l.wrap(l.data.Document.Number, codeW-4, l.size-1, "")
	title := l.wrap(domain.DocumentTypeLabelFA(l.data.Document), titleW-6, l.size+5, "B")
	h := math.Max(25, math.Max(float64(len(brand))*(l.lineHeight+1)+float64(len(subtitle))*l.lineHeight+6, float64(len(title))*l.lineHeight*1.5+8))
	h = math.Max(h, float64(len(code))*l.lineHeight+13)
	l.box(l.left, 18, codeW, h, white)
	l.box(l.left+codeW+3, 18, titleW, h, pale)
	l.text(l.left+2, 20, codeW-4, 6, "شماره سند", l.size-1, "", "C", muted)
	for i, line := range code {
		l.text(l.left+2, 27+float64(i)*l.lineHeight, codeW-4, l.lineHeight, line, l.size-1, "B", "C", l.primary)
	}
	for i, line := range title {
		l.text(l.left+codeW+6, 21+float64(i)*l.lineHeight*1.5, titleW-6, l.lineHeight*1.5, line, l.size+5, "B", "C", l.primary)
	}
	x, y := l.right-brandW, 19.0
	if logoType != "" {
		p.ImageOptions(logoPath, x, y, 20, 15, false, fpdf.ImageOptions{ImageType: logoType, ReadDpi: true}, 0, "")
		x += 24
		brandW -= 24
	}
	for _, line := range brand {
		l.text(x, y, brandW, l.lineHeight+1, line, l.size+2, "B", "R", l.primary)
		y += l.lineHeight + 1
	}
	for _, line := range subtitle {
		l.text(x, y+2, brandW, l.lineHeight, line, l.size-1, "", "R", muted)
		y += l.lineHeight
	}
	y = 18 + h + 5
	p.SetDrawColor(l.primary[0], l.primary[1], l.primary[2])
	p.SetLineWidth(0.7)
	p.Line(l.left, y, l.right, y)
	p.SetLineWidth(0.2)
	y += 4
	fields := [][2]string{{"شماره سند", l.data.Document.Number}, {"تاریخ صدور", jalaliDate(l.data.Document.IssueDate)}, {"نوع فاکتور", domain.DocumentTypeLabelFA(l.data.Document)}, {"واحد پول", currencyName(l.data.Document.Currency)}}
	w := l.width / 4
	maxH := 12.0
	for i, field := range fields {
		x := l.right - float64(i+1)*w
		l.text(x+2, y, w-4, 5, field[0], l.size-2, "", "R", muted)
		lines := l.wrap(field[1], w-4, l.size-1, "")
		for j, line := range lines {
			l.text(x+2, y+5+float64(j)*l.lineHeight, w-4, l.lineHeight, line, l.size-1, "", "R", ink)
		}
		maxH = math.Max(maxH, 5+float64(len(lines))*l.lineHeight)
	}
	p.SetY(y + maxH + 6)
}

func (l *invoiceLayout) partyLines(p invoicehtml.PartySnapshot, width float64) []string {
	values := []string{first(p.Name, p.LegalName)}
	if p.LegalName != "" && p.LegalName != p.Name {
		values = append(values, p.LegalName)
	}
	for _, field := range [][2]string{{"تلفن", p.Phone}, {"ایمیل", p.Email}, {"نشانی", p.Address}, {"شناسه مالیاتی", p.TaxID}} {
		if strings.TrimSpace(field[1]) != "" {
			values = append(values, field[0]+": "+field[1])
		}
	}
	var lines []string
	for _, value := range values {
		lines = append(lines, l.wrap(value, width, l.size-1, "")...)
	}
	return lines
}

func (l *invoiceLayout) parties(issuer, customer invoicehtml.PartySnapshot) {
	w := (l.width - 5) / 2
	rows := [][]string{l.partyLines(customer, w-8), l.partyLines(issuer, w-8)}
	maxLines := max(len(rows[0]), len(rows[1]))
	for offset := 0; offset < maxLines; {
		l.ensure(12 + l.lineHeight)
		y := l.p.GetY()
		count := min(maxLines-offset, int((l.bottom-y-12)/l.lineHeight))
		if count < 1 {
			l.nextPage()
			continue
		}
		h := 12 + float64(count)*l.lineHeight
		for col, lines := range rows {
			x := l.left + float64(col)*(w+5)
			l.box(x, y, w, h, white)
			l.box(x, y, w, 8, l.accent)
			title := "مشخصات مشتری"
			if col == 1 {
				title = "مشخصات صادرکننده"
			}
			l.text(x+4, y, w-8, 8, title, l.size, "B", "R", white)
			for j := 0; j < count && offset+j < len(lines); j++ {
				l.text(x+4, y+10+float64(j)*l.lineHeight, w-8, l.lineHeight, lines[offset+j], l.size-1, "", "R", ink)
			}
		}
		l.p.SetY(y + h + 6)
		offset += count
	}
}

func (l *invoiceLayout) tableWidths() []float64 {
	// Logical RTL order matches Excel columns A:G; amount is the leftmost cell.
	return []float64{l.width * .055, l.width * .32, l.width * .07, l.width * .075, l.width * .16, l.width * .14, l.width * .18}
}

func (l *invoiceLayout) tableHeader() {
	widths := l.tableWidths()
	headers := []string{"ردیف", "شرح کالا / خدمات", "تعداد", "واحد", "قیمت واحد", "تخفیف", "مبلغ"}
	y, x := l.p.GetY(), l.right
	for i, title := range headers {
		x -= widths[i]
		l.box(x, y, widths[i], 9, l.primary)
		l.text(x+1, y, widths[i]-2, 9, title, math.Min(l.size-1, 9), "B", "C", white)
	}
	l.p.SetY(y + 9)
}

func (l *invoiceLayout) items() {
	l.ensure(22)
	l.tableHeader()
	widths := l.tableWidths()
	for i, item := range l.data.Document.Items {
		amount, err := item.LineTotal()
		if err != nil {
			l.p.SetError(err)
			return
		}
		values := []string{faDigits(fmt.Sprint(i + 1)), item.Description, faDigits(fmt.Sprint(item.Quantity)), item.Unit, moneyLabel(item.UnitPrice), moneyLabel(item.Discount), moneyLabel(amount)}
		rows := make([][]string, len(values))
		maxLines := 1
		for j, value := range values {
			rows[j] = l.wrap(value, widths[j]-4, l.size-1, "")
			maxLines = max(maxLines, len(rows[j]))
		}
		rowHeight := math.Max(9, float64(maxLines)*l.lineHeight+4)
		if l.p.GetY()+rowHeight > l.bottom && rowHeight <= l.bottom-40 {
			l.nextPage()
			l.tableHeader()
		}
		for offset := 0; offset < maxLines; {
			if l.p.GetY()+l.lineHeight+4 > l.bottom {
				l.nextPage()
				l.tableHeader()
			}
			y, x := l.p.GetY(), l.right
			count := min(maxLines-offset, int((l.bottom-y-4)/l.lineHeight))
			if count < 1 {
				l.p.SetError(fmt.Errorf("اندازه فونت برای صفحه انتخابی بیش از حد بزرگ است"))
				return
			}
			h := math.Max(9, float64(count)*l.lineHeight+4)
			fill := white
			if i%2 == 1 {
				fill = [3]int{247, 249, 252}
			}
			for j, lines := range rows {
				x -= widths[j]
				l.box(x, y, widths[j], h, fill)
				align := "R"
				if j == 0 || j == 2 || j == 3 {
					align = "C"
				}
				for k := 0; k < count && offset+k < len(lines); k++ {
					l.text(x+2, y+2+float64(k)*l.lineHeight, widths[j]-4, l.lineHeight, lines[offset+k], l.size-1, "", align, ink)
				}
			}
			l.p.SetY(y + h)
			offset += count
		}
	}
	l.p.SetY(l.p.GetY() + 5)
}

func (l *invoiceLayout) totals() {
	values := []struct {
		label string
		value domain.Money
	}{{"جمع اقلام", l.data.Totals.Subtotal}}
	values = append(values, struct {
		label string
		value domain.Money
	}{"جمع تعداد اقلام", domain.Money(l.data.Totals.TotalQuantity)})
	if l.data.Totals.ItemDiscount > 0 {
		values = append(values, struct {
			label string
			value domain.Money
		}{"تخفیف اقلام", l.data.Totals.ItemDiscount})
	}
	if l.data.Totals.InvoiceDiscount > 0 {
		values = append(values, struct {
			label string
			value domain.Money
		}{"تخفیف کلی", l.data.Totals.InvoiceDiscount})
	}
	if l.data.Template.ShowTax {
		values = append(values, struct {
			label string
			value domain.Money
		}{"مالیات", l.data.Totals.Tax})
	}
	values = append(values, struct {
		label string
		value domain.Money
	}{"مبلغ نهایی", l.data.Totals.Total})
	for _, charge := range l.data.Document.AdditionalCharges {
		values = append(values, struct {
			label string
			value domain.Money
		}{"هزینهٔ اضافه: " + charge.Description, charge.Amount})
	}
	grand := l.data.Totals.GrandTotal
	if grand == 0 {
		grand = l.data.Totals.Total
	}
	if len(l.data.Document.AdditionalCharges) > 0 {
		values = append(values, struct {
			label string
			value domain.Money
		}{"مبلغ قابل پرداخت", grand})
	}
	h := math.Max(8, l.lineHeight+3)
	l.ensure(float64(len(values))*h + 3)
	for i, value := range values {
		l.ensure(h)
		y := l.p.GetY()
		fill, color, style := pale, l.primary, ""
		size := l.size
		if i == len(values)-1 {
			fill, color, style = l.primary, white, "B"
			size++
		}
		l.box(l.left, y, l.width, h, fill)
		// Separate wide cells prevent label/amount/currency collisions.
		amountW := math.Max(58, l.width*.38)
		l.text(l.left+4, y, amountW-8, h, moneyLabel(value.value), size, style, "L", color)
		label := value.label
		if i == len(values)-1 {
			label += " (" + currencyName(l.data.Document.Currency) + ")"
		}
		l.text(l.left+amountW, y, l.width-amountW-4, h, label, size, style, "R", color)
		l.p.SetY(y + h)
	}
	words := domain.AmountInWordsFA(grand, l.data.Document.Currency)
	l.ensure(l.lineHeight + 4)
	l.text(l.left, l.p.GetY()+2, l.width, l.lineHeight+2, "به حروف: "+words, l.size-1, "B", "R", ink)
	l.p.SetY(l.p.GetY() + l.lineHeight + 4)
}

func (l *invoiceLayout) section(title string, paragraphs []string) {
	l.p.SetY(l.p.GetY() + 6)
	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, l.wrap(paragraph, l.width-8, l.size-1, "")...)
	}
	for offset := 0; offset < len(lines); {
		l.ensure(12 + l.lineHeight)
		y := l.p.GetY()
		count := min(len(lines)-offset, int((l.bottom-y-12)/l.lineHeight))
		if count < 1 {
			l.nextPage()
			continue
		}
		h := 12 + float64(count)*l.lineHeight
		l.box(l.left, y, l.width, h, white)
		l.box(l.left, y, l.width, 8, l.accent)
		label := title
		if offset > 0 {
			label += " (ادامه)"
		}
		l.text(l.left+4, y, l.width-8, 8, label, l.size, "B", "R", white)
		for i := 0; i < count; i++ {
			l.text(l.left+4, y+10+float64(i)*l.lineHeight, l.width-8, l.lineHeight, lines[offset+i], l.size-1, "", "R", ink)
		}
		l.p.SetY(y + h)
		offset += count
	}
}
