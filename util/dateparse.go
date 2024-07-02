package util

import (
	"fmt"
	"strings"
	"time"
)

// AcceptedDateFmts is the array of string formats that StringToDate accepts
var AcceptedDateFmts = []string{
	"2006-1-2",
	"1/2/06",
	"1/2/2006",
	"2006-01-02",
	"2006/01/02",
	"2006-01-02T15:04:05",
	time.RFC822,
	time.RFC822Z,
	time.RFC1123Z,
	time.ANSIC,
	time.UnixDate,
	time.RubyDate,
	"2006-01-02T15:04:05-07:00",
}

// StringToDate tries to convert the supplied string to a time.Time value. It will use the
// formats called out in dbtypes.go:  RRDATEFMT, RRDATEINPFMT, RRDATEINPFMT2, ...
//
// for further experimentation, try: https://play.golang.org/p/JNUnA5zbMoz
// ----------------------------------------------------------------------------------
func StringToDate(s string) (time.Time, error) {
	s = Stripchars(s, "\"")
	s = strings.TrimSpace(s)
	// try the ansi std date format first
	match, Dt := easyDates(s)
	if match {
		return Dt, nil
	}
	var err error
	for i := 0; i < len(AcceptedDateFmts); i++ {
		Dt, err = time.Parse(AcceptedDateFmts[i], s)
		if nil == err {
			return Dt, nil
		}
	}
	return Dt, fmt.Errorf("date could not be decoded")
}
func easyDates(s string) (bool, time.Time) {
	match := true
	now := time.Now()
	dt := UTCDate(now)
	switch s {
	case "today":
		return match, dt
	case "yesterday":
		return match, dt.AddDate(0, 0, -1)
	case "tomorrow":
		return match, dt.AddDate(0, 0, 1)
	default:
		match = false
	}
	return match, dt
}

// UTCDate returns the current date based on the time.Time
// value supplied.  It generates a new date using only the
// year, month and day in UTC
// ----------------------------------------------------------------
func UTCDate(now time.Time) time.Time {
	dt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return dt
}
