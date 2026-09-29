package main

import (
	"testing"
	"time"
)

func TestReminders(t *testing.T) {
	at := func(date string, hour int) time.Time {
		d, _ := time.ParseInLocation(dateKey, date, time.Local)
		return d.Add(time.Duration(hour) * time.Hour)
	}
	tests := []struct {
		now  time.Time
		want []string
	}{
		{at("2026-09-25", 9), []string{"weekly-2026-09-25"}},                         // Friday
		{at("2026-09-25", 8), nil},                                                   // before 09:00
		{at("2026-09-24", 10), nil},                                                  // Thursday
		{at("2026-10-21", 9), []string{"timesheet-2026-10-21"}},                      // Wednesday the 21st
		{at("2026-11-20", 9), []string{"weekly-2026-11-20", "timesheet-2026-11-20"}}, // 21st is Saturday
		{at("2026-11-21", 9), nil},
		{at("2027-03-19", 9), []string{"weekly-2027-03-19", "timesheet-2027-03-19"}}, // 21st is Sunday
	}
	for _, tt := range tests {
		var got []string
		for _, r := range reminders(tt.now) {
			got = append(got, r.ID)
		}
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.now.Format("2006-01-02 15h"), got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.now.Format("2006-01-02 15h"), got, tt.want)
			}
		}
	}
}
