package textshape

import (
	"strings"
	"testing"
)

func TestBidiShapesPersianLetters(t *testing.T) {
	got, err := Bidi("سلام")
	if err != nil {
		t.Fatalf("Bidi returned error: %v", err)
	}
	if got == "سلام" || (!strings.ContainsRune(got, rune(0xFE91)) && !strings.ContainsRune(got, rune(0xFEE1))) {
		t.Fatalf("expected contextual Arabic presentation forms, got %U", []rune(got))
	}
}

func TestBidiKeepsMixedLatinIdentifier(t *testing.T) {
	got, err := Bidi("فاکتور INV-123")
	if err != nil {
		t.Fatalf("Bidi returned error: %v", err)
	}
	if !strings.Contains(got, "INV-123") {
		t.Fatalf("Latin identifier changed in mixed-direction text: %q", got)
	}
}
