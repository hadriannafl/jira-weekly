package main

import (
	"fmt"
	"strings"
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
		if !inWeekly(is) {
			continue
		}
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

	var reported []Issue
	for _, sec := range [][]Issue{last, current, next} {
		reported = append(reported, sec...)
	}
	parents, err := fetchParents(client, reported)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	sheet := indoDate(start.AddDate(0, 0, 4)) // Friday of that week
	f.SetSheetName("Sheet1", sheet)
	s := newWeeklyStyles(f)

	// Title banner
	_, weekNum := start.ISOWeek()
	f.MergeCell(sheet, "B1", "F1")
	f.SetCellValue(sheet, "B1", "WEEKLY REPORT")
	f.SetCellStyle(sheet, "B1", "F1", s.title)
	f.SetRowHeight(sheet, 1, 36)
	f.MergeCell(sheet, "B2", "F2")
	f.SetCellValue(sheet, "B2", fmt.Sprintf("%s   ·   Minggu ke-%d   ·   %s – %s",
		client.DisplayName, weekNum, indoDate(start), indoDate(start.AddDate(0, 0, 4))))
	f.SetCellStyle(sheet, "B2", "F2", s.subtitle)
	f.SetRowHeight(sheet, 2, 24)

	row := 4
	for _, sec := range []struct {
		title  string
		color  string
		issues []Issue
	}{
		{"LAST WEEK", "64748B", last},
		{"CURRENT WEEK", "4F46E5", current},
		{"NEXT WEEK", "D97706", next},
	} {
		f.MergeCell(sheet, cell("B", row), cell("F", row))
		f.SetCellValue(sheet, cell("B", row), fmt.Sprintf("%s  ·  %d tiket", sec.title, len(sec.issues)))
		f.SetCellStyle(sheet, cell("B", row), cell("F", row), s.section(sec.color))
		f.SetRowHeight(sheet, row, 26)
		row++

		f.SetSheetRow(sheet, cell("B", row), &[]interface{}{"No", "Ticket No.", "Ticket Desc", "Status", "Note"})
		f.SetCellStyle(sheet, cell("B", row), cell("F", row), s.header)
		f.SetCellStyle(sheet, cell("B", row), cell("B", row), s.headerCenter)
		f.SetCellStyle(sheet, cell("E", row), cell("E", row), s.headerCenter)
		f.SetRowHeight(sheet, row, 22)
		row++

		if len(sec.issues) == 0 {
			f.MergeCell(sheet, cell("B", row), cell("F", row))
			f.SetCellValue(sheet, cell("B", row), "Tidak ada tiket")
			f.SetCellStyle(sheet, cell("B", row), cell("F", row), s.empty)
			f.SetRowHeight(sheet, row, 22)
			row++
		}
		for i, is := range sec.issues {
			z := i % 2
			status := weeklyStatus(is, parents)
			f.SetSheetRow(sheet, cell("B", row), &[]interface{}{i + 1, is.Key, is.Fields.Summary, status, weeklyNote(is, status)})
			f.SetCellHyperLink(sheet, cell("C", row), client.BaseURL+"/browse/"+is.Key, "External")
			f.SetCellStyle(sheet, cell("B", row), cell("B", row), s.no[z])
			f.SetCellStyle(sheet, cell("C", row), cell("C", row), s.link[z])
			f.SetCellStyle(sheet, cell("D", row), cell("D", row), s.text[z])
			f.SetCellStyle(sheet, cell("E", row), cell("E", row), s.status(status))
			f.SetCellStyle(sheet, cell("F", row), cell("F", row), s.text[z])
			f.SetRowHeight(sheet, row, max(22, 16*wrappedLines([]string{is.Fields.Summary}, 68)))
			row++
		}
		row++
	}

	for col, w := range map[string]float64{"A": 2, "B": 6, "C": 14, "D": 70, "E": 14, "F": 45} {
		f.SetColWidth(sheet, col, col, w)
	}
	noGrid, landscape, fit, one := false, "landscape", true, 1
	f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &noGrid})
	f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fit})
	f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: &landscape, FitToWidth: &one})
	return f, weeklyFileName(start), nil
}

type weeklyStyles struct {
	f                                     *excelize.File
	title, subtitle, header, headerCenter int
	empty                                 int
	no, link, text                        [2]int // index 0/1 = zebra rows
	done, todo, qc, tr, wip               int
	sections                              map[string]int
}

func newWeeklyStyles(f *excelize.File) *weeklyStyles {
	fill := func(c string) excelize.Fill {
		return excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{c}}
	}
	line := []excelize.Border{{Type: "bottom", Color: "E5E7EB", Style: 1}}
	style := func(st excelize.Style) int {
		id, _ := f.NewStyle(&st)
		return id
	}
	pill := func(bg, fg string, numFmt int) int {
		return style(excelize.Style{
			Fill:      fill(bg),
			Font:      &excelize.Font{Bold: true, Color: fg},
			Border:    []excelize.Border{{Type: "bottom", Color: "FFFFFF", Style: 2}, {Type: "top", Color: "FFFFFF", Style: 2}},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
			NumFmt:    numFmt,
		})
	}

	s := &weeklyStyles{f: f, sections: map[string]int{}}
	s.title = style(excelize.Style{
		Fill:      fill("4F46E5"),
		Font:      &excelize.Font{Bold: true, Size: 18, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
	})
	s.subtitle = style(excelize.Style{
		Fill:      fill("EEF2FF"),
		Font:      &excelize.Font{Size: 11, Color: "4338CA"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
	})
	s.header = style(excelize.Style{
		Fill:      fill("1E293B"),
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
	})
	s.headerCenter = style(excelize.Style{
		Fill:      fill("1E293B"),
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	s.empty = style(excelize.Style{
		Font:      &excelize.Font{Italic: true, Color: "94A3B8"},
		Border:    line,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	for z, bg := range []string{"FFFFFF", "F8FAFC"} {
		s.no[z] = style(excelize.Style{
			Fill: fill(bg), Border: line, Font: &excelize.Font{Color: "64748B"},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		s.link[z] = style(excelize.Style{
			Fill: fill(bg), Border: line, Font: &excelize.Font{Bold: true, Color: "4F46E5", Underline: "single"},
			Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
		})
		s.text[z] = style(excelize.Style{
			Fill: fill(bg), Border: line, Font: &excelize.Font{Color: "0F172A"},
			Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1, WrapText: true},
		})
	}
	s.done = pill("DCFCE7", "166534", 9)
	s.todo = pill("F1F5F9", "475569", 9)
	s.qc = pill("FEF3C7", "92400E", 0)
	s.tr = pill("EDE9FE", "5B21B6", 0)
	s.wip = pill("DBEAFE", "1E40AF", 0)
	return s
}

// section is the colored bar above each table, tinted with color.
func (s *weeklyStyles) section(color string) int {
	if id, ok := s.sections[color]; ok {
		return id
	}
	id, _ := s.f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: color},
		Border:    []excelize.Border{{Type: "left", Color: color, Style: 5}, {Type: "bottom", Color: color, Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", Indent: 1},
	})
	s.sections[color] = id
	return id
}

// status picks the colored pill for a Status value from weeklyStatus.
func (s *weeklyStyles) status(v interface{}) int {
	switch v := v.(type) {
	case int:
		if v == 1 {
			return s.done
		}
		return s.todo
	case string:
		switch strings.ToUpper(v) {
		case "QC":
			return s.qc
		case "TR":
			return s.tr
		}
	}
	return s.wip
}

// inWeekly keeps only Dev and Bug tickets in the weekly report (TR and other
// types are left out).
func inWeekly(is Issue) bool {
	t := strings.ToLower(is.Fields.IssueType.Name)
	return strings.Contains(t, "dev") || strings.Contains(t, "bug")
}

// weeklyStatus is 0% for to do and the Jira status name while in progress.
// A finished Dev ticket shows the stage it is waiting on ("QC", then "TR")
// and only becomes 100% once those sibling subtasks are done too.
func weeklyStatus(is Issue, parents map[string]Issue) interface{} {
	switch is.Fields.Status.StatusCategory.Key {
	case "new":
		return 0
	case "done":
		if hasWord(is.Fields.IssueType.Name, "dev") {
			if stage := pendingStage(is, parents); stage != "" {
				return stage
			}
		}
		return 1
	}
	return is.Fields.Status.Name
}

// weeklyNote writes the Note column from the Status value, in the same
// wording the report used when it was filled by hand.
func weeklyNote(is Issue, status interface{}) string {
	bug := hasWord(is.Fields.IssueType.Name, "bug")
	switch v := status.(type) {
	case int:
		switch {
		case v == 1 && bug:
			return "DONE - Bug sudah diperbaiki"
		case v == 1:
			return "DONE - Selesai"
		case bug:
			return "Rencana perbaikan bug minggu depan"
		}
		return "Rencana development minggu depan"
	case string:
		switch strings.ToUpper(v) {
		case "QC":
			return "DONE - Sudah submit ke QC, menunggu hasil pengujian"
		case "TR":
			return "QC selesai, menunggu TR"
		}
		if strings.EqualFold(v, "In Progress") {
			if bug {
				return "DEV - Sedang perbaikan bug"
			}
			return "DEV - Sedang dikerjakan"
		}
		return "DEV - " + v
	}
	return ""
}

// pendingStage returns "QC" or "TR" while that subtask of the same parent is
// not done yet. Without a known parent the next step is assumed to be QC.
func pendingStage(is Issue, parents map[string]Issue) string {
	p, ok := parents[parentKey(is)]
	if !ok {
		return "QC"
	}
	for _, stage := range []string{"qc", "tr"} {
		for _, st := range p.Fields.Subtasks {
			if hasWord(st.Fields.IssueType.Name, stage) && st.Fields.Status.StatusCategory.Key != "done" {
				return strings.ToUpper(stage)
			}
		}
	}
	return ""
}

// hasWord reports whether name contains word as a separate word, ignoring
// case ("Sub-task TR" has "tr", "Story" does not).
func hasWord(name, word string) bool {
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '/'
	}) {
		if w == word {
			return true
		}
	}
	return false
}

func cell(col string, row int) string {
	return fmt.Sprintf("%s%d", col, row)
}
