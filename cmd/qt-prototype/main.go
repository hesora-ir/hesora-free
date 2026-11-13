package main

import (
	"fmt"
	"os"

	qt "github.com/mappu/miqt/qt6"
)

func main() {
	qt.NewQApplication(os.Args)
	qt.QGuiApplication_SetLayoutDirection(qt.RightToLeft)

	window := qt.NewQMainWindow2()
	window.QWidget.SetWindowTitle("فاکتور ساز — نمونه فنی فاز ۱")
	window.QWidget.SetFixedSize2(560, 260)

	root := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	root.SetLayout(layout.QLayout)

	title := qt.NewQLabel(window.QWidget)
	title.SetText("نمونه اولیه اتصال Go و Qt 6")
	title.SetAlignment(qt.AlignCenter)
	layout.AddWidget(title.QWidget)

	status := qt.NewQLabel(window.QWidget)
	status.SetText("برای بررسی رویدادهای رابط کاربری، دکمه را انتخاب کنید.")
	status.SetAlignment(qt.AlignCenter)
	layout.AddWidget(status.QWidget)

	clickCount := 0
	button := qt.NewQPushButton5("آزمایش رویداد", window.QWidget)
	button.OnClicked(func() {
		clickCount++
		status.SetText(fmt.Sprintf("رویداد با موفقیت دریافت شد: %d", clickCount))
	})
	layout.AddWidget(button.QWidget)

	window.SetCentralWidget(root)
	window.Show()
	qt.QApplication_Exec()
}
