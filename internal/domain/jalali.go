package domain

import "errors"

type JalaliDate struct{ Year, Month, Day int }

func (d JalaliDate) Validate() error {
	if d.Year < 1 || d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > JalaliMonthLength(d.Year, d.Month) {
		return errors.New("تاریخ شمسی نامعتبر است")
	}
	return nil
}

func JalaliMonthLength(year, month int) int {
	if month < 1 || month > 12 {
		return 0
	}
	if month <= 6 {
		return 31
	}
	if month <= 11 {
		return 30
	}
	if IsJalaliLeapYear(year) {
		return 30
	}
	return 29
}

// IsJalaliLeapYear uses the 33-year arithmetic cycle used by the application's
// initial calendar implementation. Calendar boundaries are tested explicitly.
func IsJalaliLeapYear(year int) bool {
	if year < 1 {
		return false
	}
	r := year % 33
	return r == 1 || r == 5 || r == 9 || r == 13 || r == 17 || r == 22 || r == 26 || r == 30
}

func (d JalaliDate) ToGregorian() (year, month, day int, err error) {
	if err = d.Validate(); err != nil {
		return
	}
	jy := d.Year + 1595
	days := -355668 + 365*jy + (jy/33)*8 + ((jy%33 + 3) / 4) + d.Day
	if d.Month < 7 {
		days += (d.Month - 1) * 31
	} else {
		days += (d.Month-7)*30 + 186
	}
	year = 400 * (days / 146097)
	days %= 146097
	if days > 36524 {
		year += 100 * ((days - 1) / 36524)
		days = (days - 1) % 36524
		if days >= 365 {
			days++
		}
	}
	year += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		year += (days - 1) / 365
		days = (days - 1) % 365
	}
	day = days + 1
	leap := year%4 == 0 && (year%100 != 0 || year%400 == 0)
	monthDays := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if leap {
		monthDays[1] = 29
	}
	for month = 1; month <= 12 && day > monthDays[month-1]; month++ {
		day -= monthDays[month-1]
	}
	return year, month, day, nil
}

func GregorianToJalali(gy, gm, gd int) (JalaliDate, error) {
	monthDays := [...]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	if gy < 1 || gm < 1 || gm > 12 {
		return JalaliDate{}, errors.New("تاریخ میلادی نامعتبر است")
	}
	leap := gy%4 == 0 && (gy%100 != 0 || gy%400 == 0)
	maxDay := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[gm-1]
	if gm == 2 && leap {
		maxDay = 29
	}
	if gd < 1 || gd > maxDay {
		return JalaliDate{}, errors.New("تاریخ میلادی نامعتبر است")
	}
	gy2 := gy
	if gm > 2 {
		gy2++
	}
	days := 355666 + 365*gy + (gy2+3)/4 - (gy2+99)/100 + (gy2+399)/400 + gd + monthDays[gm-1]
	jy := -1595 + 33*(days/12053)
	days %= 12053
	jy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}
	var jm, jd int
	if days < 186 {
		jm = 1 + days/31
		jd = 1 + days%31
	} else {
		jm = 7 + (days-186)/30
		jd = 1 + (days-186)%30
	}
	return JalaliDate{Year: jy, Month: jm, Day: jd}, nil
}
