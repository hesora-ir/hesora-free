package domain

import "testing"

func TestJalaliDateConversion(t *testing.T) {
	j := JalaliDate{Year: 1403, Month: 1, Day: 1}
	y, m, d, err := j.ToGregorian()
	if err != nil || y != 2024 || m != 3 || d != 20 {
		t.Fatalf("got %04d-%02d-%02d, err=%v", y, m, d, err)
	}
	got, err := GregorianToJalali(2024, 3, 20)
	if err != nil || got != j {
		t.Fatalf("got %+v, err=%v", got, err)
	}
}

func TestJalaliDateValidation(t *testing.T) {
	for _, d := range []JalaliDate{{1403, 0, 1}, {1403, 13, 1}, {1403, 7, 31}, {1402, 12, 30}} {
		if d.Validate() == nil {
			t.Errorf("expected invalid date: %+v", d)
		}
	}
	if (JalaliDate{1403, 12, 30}).Validate() != nil {
		t.Fatal("expected leap-year Esfand 30 to be valid")
	}
}
