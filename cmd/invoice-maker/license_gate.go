package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/cloud"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	qt "github.com/mappu/miqt/qt6"
)

func isTrialLicense(result licensing.Result) bool {
	return result.Claims != nil && strings.HasPrefix(result.Claims.Subject, "trial:")
}

func serverClock(serverTime int64, receivedAt time.Time) func() time.Time {
	return func() time.Time { return time.Unix(serverTime, 0).Add(time.Since(receivedAt)) }
}

// showLicenseGate blocks access until a server-signed license or device trial is saved.
func parseInstallerLicenseArgs(args []string) (string, bool) {
	var code string
	trial := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activate-license":
			if i+1 < len(args) {
				i++
				candidate := args[i]
				if len(candidate) >= 16 && len(candidate) <= 4096 {
					valid := true
					for _, ch := range candidate {
						if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '_' && ch != '-' && ch != '.' {
							valid = false
							break
						}
					}
					if valid {
						code = candidate
					}
				}
			}
		case "--start-trial":
			trial = true
		}
	}
	return code, trial
}

func showLicenseGate(files licensing.FileStore, installationID string, keys map[string]ed25519.PublicKey, installerCode string, installerTrial bool) (map[string]ed25519.PublicKey, string, licensing.Result, time.Duration, func() time.Time, bool) {
	dialog := qt.NewQDialog(nil)
	dialog.SetWindowTitle("فعال‌سازی هسورا")
	dialog.SetMinimumWidth(480)
	layout := qt.NewQVBoxLayout2()
	layout.AddWidget(qt.NewQLabel5("برای استفاده از هسورا، کد لایسنس را وارد کنید یا دورهٔ آزمایشی ۳۰روزه را شروع کنید.", dialog.QWidget).QWidget)
	code := qt.NewQLineEdit(dialog.QWidget)
	code.SetPlaceholderText("کد لایسنس خریداری‌شده")
	if installerCode != "" {
		code.SetText(installerCode)
	}
	layout.AddWidget(code.QWidget)
	activate := qt.NewQPushButton5("بررسی و فعال‌سازی لایسنس", dialog.QWidget)
	trial := qt.NewQPushButton5("شروع دورهٔ آزمایشی ۳۰روزه", dialog.QWidget)
	layout.AddWidget(activate.QWidget)
	layout.AddWidget(trial.QWidget)
	status := qt.NewQLabel5("", dialog.QWidget)
	status.SetWordWrap(true)
	layout.AddWidget(status.QWidget)
	dialog.SetLayout(layout.QLayout)
	accepted := false
	trialRemaining := time.Duration(0)
	var trialClock func() time.Time
	complete := func(trialMode bool) {
		status.SetText("در حال بررسی امن با سرور Hesora…")
		activate.SetEnabled(false)
		trial.SetEnabled(false)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client, err := cloud.New(hesoraCloudURL)
		var license cloud.License
		if err == nil {
			if trialMode {
				var fingerprint string
				fingerprint, err = licensing.DeviceFingerprint()
				if err == nil {
					license, err = client.StartTrial(ctx, installationID, fingerprint)
				}
			} else {
				license, err = client.ActivateLicense(ctx, strings.TrimSpace(code.Text()), installationID)
			}
		}
		if err == nil && trialMode && license.ServerTime > 0 {
			receivedAt := time.Now()
			trialClock = serverClock(license.ServerTime, receivedAt)
			trialRemaining = time.Duration(license.ExpiresAt-license.ServerTime) * time.Second
		}
		var received map[string]ed25519.PublicKey
		if err == nil {
			received, err = cloud.DecodePublicKeys(license.PublicKeys)
		}
		if err != nil && license.Token != "" {
			// Keep compatibility with servers returning a token before the key-ring response.
			received, err = client.PublicKeys(ctx)
		}
		if err == nil && len(received) == 0 {
			err = errors.New("کلید عمومی اعتبارسنجی از سرور دریافت نشد.")
		}
		if err == nil {
			verifyNow := time.Now
			if trialMode && trialClock != nil {
				verifyNow = trialClock
			}
			verifier := licensing.Verifier{PublicKeys: received, InstallationID: installationID, Now: verifyNow}
			result := verifier.Verify(license.Token)
			if !result.HasFeature(licensing.ProFullFeature) {
				err = result.Reason
				if err == nil {
					err = errors.New("لایسنس دسترسی کامل ندارد.")
				}
			}
		}
		if err == nil {
			err = files.SavePublicKeys(received)
		}
		if err == nil {
			err = files.SaveToken(license.Token)
		}
		if err != nil {
			status.SetText("فعال‌سازی انجام نشد: " + err.Error())
			activate.SetEnabled(true)
			trial.SetEnabled(true)
			return
		}
		accepted = true
		keys = received
		dialog.Accept()
	}
	activate.OnClicked(func() {
		if strings.TrimSpace(code.Text()) == "" {
			status.SetText("کد لایسنس را وارد کنید یا دورهٔ آزمایشی را انتخاب کنید.")
			return
		}
		complete(false)
	})
	trial.OnClicked(func() { complete(true) })
	if installerCode != "" || installerTrial {
		autoActivate := qt.NewQTimer2(dialog.QObject)
		autoActivate.OnTimeout(func() {
			autoActivate.Stop()
			if installerCode != "" {
				complete(false)
			} else if installerTrial {
				complete(true)
			}
		})
		autoActivate.Start(100)
	}
	dialog.Exec()
	token, _ := files.ReadToken()
	verifyNow := time.Now
	if trialClock != nil {
		verifyNow = trialClock
	}
	result := (licensing.Verifier{PublicKeys: keys, InstallationID: installationID, Now: verifyNow}).Verify(token)
	return keys, token, result, trialRemaining, trialClock, accepted && result.HasFeature(licensing.ProFullFeature)
}
