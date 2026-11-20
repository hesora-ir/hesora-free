//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		return
	}
	if err := exec.Command(os.Args[1], os.Args[2:]...).Start(); err != nil {
		showError("اجرای هسورا پس از نصب انجام نشد. برنامه را از میان‌بر دسکتاپ اجرا کنید.\n" + err.Error())
	}
}

func showError(message string) {
	text, errText := syscall.UTF16PtrFromString(message)
	title, errTitle := syscall.UTF16PtrFromString("اجرای هسورا ناموفق بود")
	if errText != nil || errTitle != nil {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
