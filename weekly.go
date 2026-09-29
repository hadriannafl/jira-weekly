package main

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

func weeklyFileName(start time.Time) string {
	year, weekNum := start.ISOWeek()
	return fmt.Sprintf("REPORT_WEEKLY_TASK_%d%02d.xlsx", year, weekNum)
}

// buildWeekly makes the weekly report in the LAST WEEK / CURRENT WEEK /
// NEXT WEEK layout. The caller must Close the file.
func buildWeekly(client *Client, week int) (*excelize.File, string, error) {
	start, end := weekRange(time.Now(), week)
	prev := start.AddDate(0, 0, -7)
	issues, err := fetchActivity(client, prev)
	if err != nil {
		return nil, "", err
	}

	var last, current, next []Issue
	for _, is := range issues {
		days := activityDays(is, client.AccountID)
		cat := is.Fields.Status.StatusCategory.Key
		if activeBetween(days, prev, start) {
			last = append(last, is)
		}
		if activeBetween(days, start, end) || cat == "indeterminate" {
			current = append(current, is)
		}
		if cat == "new" {
			next = append(next, is)
		}
	}

	f := excelize.NewFile()
	sheet := indoDate(start.AddDate(0, 0, 4)) // Friday of that week
	f.SetSheetName("Sheet1", sheet)

	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	header, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFF00"}},
	})
	percent, _ := f.NewStyle(&excelize.Style{NumFmt: 9})

	row := 1
	for _, sec := range []struct {
		title  string
		issues []Issue
	}{{"LAST WEEK", last}, {"CURRENT WEEK", current}, {"NEXT WEEK", next}} {
		f.SetCellValue(sheet, cell("A", row), sec.title)
		f.SetCellStyle(sheet, cell("A", row), cell("A", row), bold)
		f.SetSheetRow(sheet, cell("B", row), &[]interface{}{"No", "Ticket No.", "Ticket Desc", "Status", "Note"})
		f.SetCellStyle(sheet, cell("B", row), cell("F", row), header)
		for i, is := range sec.issues {
			r := row + 1 + i
			f.SetSheetRow(sheet, cell("B", r), &[]interface{}{i + 1, is.Key, is.Fields.Summary, weeklyStatus(is)})
			f.SetCellStyle(sheet, cell("E", r), cell("E", r), percent)
		}
		row += len(sec.issues) + 3
	}

	for col, w := range map[string]float64{"A": 15.7, "B": 3.6, "C": 10.4, "D": 69.6, "E": 6.7, "F": 47.7} {
		f.SetColWidth(sheet, col, col, w)
	}
	return f, weeklyFileName(start), nil
}

// weeklyStatus is 100% for done, 0% for to do, otherwise the Jira status
// name (e.g. "QC"), which you can overwrite with a percentage.
func weeklyStatus(is Issue) interface{} {
	switch is.Fields.Status.StatusCategory.Key {
	case "done":
		return 1
	case "new":
		return 0
	}
	return is.Fields.Status.Name
}

func cell(col string, row int) string {
	return fmt.Sprintf("%s%d", col, row)
}
