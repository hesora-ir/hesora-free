package main

import (
	"fmt"
	"strconv"

	"github.com/hesora-ir/hesora-free/internal/domain"
	qt "github.com/mappu/miqt/qt6"
)

// attachInvoiceItemSuggestions keeps the popup label separate from the saved description.
func attachInvoiceItemSuggestions(row *qt.QWidget, name, unit, price *qt.QLineEdit, currency string, find func(string) ([]domain.InvoiceItem, error), update func()) *qt.QCompleter {
	model := qt.NewQStringListModel3(row.QObject)
	completer := qt.NewQCompleter5(model.QAbstractItemModel, row.QObject)
	completer.SetFilterMode(qt.MatchContains)
	name.SetCompleter(completer)
	suggestions := map[string]domain.InvoiceItem{}
	name.OnTextEdited(func(text string) {
		// QLineEdit may emit textEdited while inserting the selected popup label.
		if _, selected := suggestions[text]; selected {
			return
		}
		items, err := find(text)
		if err != nil {
			suggestions = map[string]domain.InvoiceItem{}
			model.SetStringList(nil)
			return
		}
		labels := make([]string, 0, len(items))
		suggestions = map[string]domain.InvoiceItem{}
		for _, candidate := range items {
			label := fmt.Sprintf("%s — %s %s", candidate.Description, formatInvoiceMoney(int64(candidate.UnitPrice)), currency)
			labels = append(labels, label)
			suggestions[label] = candidate
		}
		model.SetStringList(labels)
	})
	// Apply after QLineEdit's own completion handler, which inserts the popup label.
	timer := qt.NewQTimer2(row.QObject)
	timer.SetSingleShot(true)
	var pending domain.InvoiceItem
	var selectedLabel string
	timer.OnTimeout(func() {
		if name.Text() != selectedLabel {
			return
		}
		name.SetText(pending.Description)
		unit.SetText(pending.Unit)
		price.SetText(strconv.FormatInt(int64(pending.UnitPrice), 10))
		update()
	})
	completer.OnActivated(func(label string) {
		if candidate, ok := suggestions[label]; ok {
			pending, selectedLabel = candidate, label
			timer.Start(0)
		}
	})
	return completer
}
