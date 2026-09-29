package main

import (
	"fmt"
	"time"
)

const reminderHour = 9

type reminder struct {
	ID       string
	Title    string
	Text     string
	Link     string
	LinkText string
}

// reminders returns the dashboard banners due at now: the weekly report on
// Fridays and the timesheet on the 21st, both from 09:00.
func reminders(now time.Time) []reminder {
	if now.Hour() < reminderHour {
		return nil
	}
	today := now.Format(dateKey)
	var rs []reminder
	if now.Weekday() == time.Friday {
		_, week := now.ISOWeek()
		rs = append(rs, reminder{
			ID:       "weekly-" + today,
			Title:    "Waktunya kirim Weekly Report",
			Text:     fmt.Sprintf("Hari ini Jumat. Lengkapi persen progres dan note, lalu kirim laporan minggu ke-%d.", week),
			Link:     "/download",
			LinkText: "Download Weekly",
		})
	}
	if due := timesheetDue(now); due.Format(dateKey) == today {
		end := time.Date(now.Year(), now.Month(), 21, 0, 0, 0, 0, time.Local)
		rs = append(rs, reminder{
			ID:       "timesheet-" + today,
			Title:    "Waktunya kirim Timesheet",
			Text:     fmt.Sprintf("Periode %s - %s berakhir. Isi jam Check In/Out, lalu kirim timesheet.", indoDate(end.AddDate(0, -1, 1)), indoDate(end)),
			Link:     "/timesheet?period=" + end.Format("2006-01"),
			LinkText: "Download Timesheet",
		})
	}
	return rs
}

// timesheetDue is the 21st of now's month, moved back to Friday when it
// falls on a weekend.
func timesheetDue(now time.Time) time.Time {
	due := time.Date(now.Year(), now.Month(), 21, 0, 0, 0, 0, time.Local)
	switch due.Weekday() {
	case time.Saturday:
		due = due.AddDate(0, 0, -1)
	case time.Sunday:
		due = due.AddDate(0, 0, -2)
	}
	return due
}
