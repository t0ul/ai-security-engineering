package dateparse

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTimesRangeSharedAMPM(t *testing.T) {
	// "5:30-8:00 PM": the earlier time inherits the later token's pm.
	got := ParseTimes("doors 5:30-8:00 PM in the gym")
	want := []HM{{17, 30}, {20, 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTimes = %v, want %v", got, want)
	}
}

func TestParseTimesRejectsBareIntegers(t *testing.T) {
	for _, s := range []string{"Room 205", "Grades 2-5", "see page 7"} {
		if got := ParseTimes(s); len(got) != 0 {
			t.Errorf("ParseTimes(%q) = %v, want none", s, got)
		}
	}
}

func TestParseTimesNoonMidnight(t *testing.T) {
	if got := ParseTimes("12:00 PM"); !reflect.DeepEqual(got, []HM{{12, 0}}) {
		t.Errorf("noon: got %v", got)
	}
	if got := ParseTimes("12:00 AM"); !reflect.DeepEqual(got, []HM{{0, 0}}) {
		t.Errorf("midnight: got %v", got)
	}
	if got := ParseTimes("8:30 AM"); !reflect.DeepEqual(got, []HM{{8, 30}}) {
		t.Errorf("morning: got %v", got)
	}
}

func TestWeekdayMismatchIntegrity(t *testing.T) {
	dp := ParseDatePhrase("Back to School Night, Thursday, September 29", DefaultYear)
	if dp == nil {
		t.Fatal("expected a date phrase")
	}
	if dp.ActualWeekday != "Tuesday" {
		t.Fatalf("2026-09-29 should be Tuesday, got %s", dp.ActualWeekday)
	}
	if !dp.Mismatch || dp.StatedWeekday != "Thursday" {
		t.Fatalf("expected stated Thursday mismatch, got stated=%q mismatch=%v", dp.StatedWeekday, dp.Mismatch)
	}
	if !strings.Contains(dp.Warning, "weekday_mismatch") {
		t.Errorf("warning not set: %q", dp.Warning)
	}
}

func TestExtractDatetimeWithRange(t *testing.T) {
	r := ExtractDatetime("September 29 2026 from 5:30-8:00 PM", DefaultYear)
	if r == nil {
		t.Fatal("expected a result")
	}
	if r.AllDay {
		t.Error("should not be all-day")
	}
	if r.Start != "2026-09-29T17:30:00" || r.End != "2026-09-29T20:00:00" {
		t.Fatalf("got start=%q end=%q", r.Start, r.End)
	}
}

func TestExtractDatetimeAllDayAndAssumedYear(t *testing.T) {
	r := ExtractDatetime("Picture Day is October 10", DefaultYear)
	if r == nil || !r.AllDay {
		t.Fatalf("expected an all-day result, got %+v", r)
	}
	if r.Start != "2026-10-10" {
		t.Errorf("start = %q", r.Start)
	}
	foundAssumed := false
	for _, w := range r.Warnings {
		if strings.HasPrefix(w, "assumed_year:2026") {
			foundAssumed = true
		}
	}
	if !foundAssumed {
		t.Errorf("expected assumed_year warning, got %v", r.Warnings)
	}
}

func TestInvalidDateRejected(t *testing.T) {
	if dp := ParseDatePhrase("February 30 2026", DefaultYear); dp != nil {
		t.Errorf("February 30 should be rejected, got %+v", dp)
	}
	if ExtractDatetime("the meeting on February 30", DefaultYear) != nil {
		t.Error("extract should be nil for an impossible date")
	}
}

func TestNoDate(t *testing.T) {
	if ExtractDatetime("there is no date in this sentence", DefaultYear) != nil {
		t.Error("expected nil when no date present")
	}
}
