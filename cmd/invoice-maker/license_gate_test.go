package main

import "testing"

func TestParseInstallerLicenseArgs(t *testing.T) {
	code := "eyJhbGciOiJFZERTQSJ9.payload.signature"
	gotCode, gotTrial := parseInstallerLicenseArgs([]string{"--activate-license", code})
	if gotCode != code || gotTrial {
		t.Fatalf("license option = (%q, %t)", gotCode, gotTrial)
	}
	gotCode, gotTrial = parseInstallerLicenseArgs([]string{"--start-trial"})
	if gotCode != "" || !gotTrial {
		t.Fatalf("trial option = (%q, %t)", gotCode, gotTrial)
	}
	gotCode, gotTrial = parseInstallerLicenseArgs([]string{"--activate-license", "bad code"})
	if gotCode != "" || gotTrial {
		t.Fatalf("invalid code must be ignored, got (%q, %t)", gotCode, gotTrial)
	}
}
