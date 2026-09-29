package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type timesheetConfig struct {
	Name         string
	Department   string
	Project      string
	Company      string
	CompanyLines []string
	AssetsDir    string
}

func loadTimesheetConfig(client *Client) timesheetConfig {
	cfg := timesheetConfig{
		Name:       os.Getenv("TS_NAME"),
		Department: os.Getenv("TS_DEPARTMENT"),
		Project:    os.Getenv("TS_PROJECT"),
		Company:    os.Getenv("TS_COMPANY"),
		AssetsDir:  os.Getenv("TS_ASSETS"),
	}
	if cfg.Name == "" {
		cfg.Name = client.DisplayName
	}
	if cfg.AssetsDir == "" {
		cfg.AssetsDir = "assets"
	}
	for _, l := range strings.Split(os.Getenv("TS_COMPANY_LINES"), "|") {
		if l = strings.TrimSpace(l); l != "" {
			cfg.CompanyLines = append(cfg.CompanyLines, l)
		}
	}
	return cfg
}

// timesheetPeriod returns the 22nd of the previous month through the 21st of
// the given month ("2006-01"). Empty period means the running period.
func timesheetPeriod(now time.Time, period string) (time.Time, time.Time, error) {
	y, m := now.Year(), now.Month()
	if period == "" {
		if now.Day() > 21 {
			m++
		}
	} else {
		t, err := time.Parse("2006-01", period)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("format period harus YYYY-MM, contoh 2026-09")
		}
		y, m = t.Year(), t.Month()
	}
	end := time.Date(y, m, 21, 0, 0, 0, 0, time.Local)
	start := time.Date(y, m-1, 22, 0, 0, 0, 0, time.Local)
	return start, end, nil
}

// buildTimesheet fills one row per day of the period. Task List comes from
// the Jira changes you made that day; Check In/Out stay empty for you to
// fill in. The caller must Close the file.
func buildTimesheet(client *Client, cfg timesheetConfig, period string) (*excelize.File, string, error) {
	start, end, err := timesheetPeriod(time.Now(), period)
	if err != nil {
		return nil, "", err
	}
	issues, err := fetchActivity(client, start)
	if err != nil {
		return nil, "", err
	}
	byDay := map[string][]Issue{}
	for _, is := range issues {
		for d := range activityDays(is, client.AccountID) {
			byDay[d] = append(byDay[d], is)
		}
	}

	f := excelize.NewFile()
	const sheet = "Sheet1"

	border := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1},
	}
	companyStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 18, Color: "E31C25"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	addrStyle, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Horizontal: "center"}})
	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10}})
	headStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"404040"}},
		Border:    border,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	center, _ := f.NewStyle(&excelize.Style{
		Border:    border,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	left, _ := f.NewStyle(&excelize.Style{
		Border:    border,
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
	})
	sumStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10}, Border: border})
	signStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	for col, w := range map[string]float64{"A": 6.1, "B": 10.4, "C": 6.9, "D": 7.3, "E": 3.9, "F": 20.7, "G": 25, "H": 20.3} {
		f.SetColWidth(sheet, col, col, w)
	}

	// Letterhead
	addImage(f, sheet, "A1", filepath.Join(cfg.AssetsDir, "logo.png"), 134, 0, 10)
	addImage(f, sheet, "H1", filepath.Join(cfg.AssetsDir, "logo-corner.png"), 96, 46, 0)
	f.MergeCell(sheet, "D2", "G2")
	f.SetCellValue(sheet, "D2", cfg.Company)
	f.SetCellStyle(sheet, "D2", "G2", companyStyle)
	f.SetRowHeight(sheet, 2, 23.25)
	for i, l := range cfg.CompanyLines {
		r := 3 + i
		f.MergeCell(sheet, cell("D", r), cell("G", r))
		f.SetCellValue(sheet, cell("D", r), l)
		f.SetCellStyle(sheet, cell("D", r), cell("G", r), addrStyle)
	}

	for i, kv := range [][2]string{
		{"Nama ", cfg.Name},
		{"Departemen ", cfg.Department},
		{"Nama Project", cfg.Project},
		{"Periode ", indoDate(start) + " - " + indoDate(end)},
	} {
		r := 8 + i
		f.MergeCell(sheet, cell("A", r), cell("B", r))
		f.SetCellValue(sheet, cell("A", r), kv[0])
		f.SetCellStyle(sheet, cell("A", r), cell("B", r), labelStyle)
		f.SetCellValue(sheet, cell("C", r), ": "+kv[1])
	}

	f.SetSheetRow(sheet, "A13", &[]interface{}{"No.", "Date", "Check In", "Check Out", "", "Work Place", "Task List", "Notes"})
	f.SetCellStyle(sheet, "A13", "H13", headStyle)

	r := 14
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		values := []interface{}{r - 13, d.Format("02-01-2006"), "-", "-", "Off", cfg.Project, "N/A", "N/A"}
		height := 30.0
		taskStyle := center
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			taskStyle = left
			var tasks, notes []string
			for _, is := range byDay[d.Format(dateKey)] {
				tasks = append(tasks, is.Key+" - "+is.Fields.Summary)
				notes = append(notes, is.Fields.Status.Name)
			}
			values = []interface{}{r - 13, d.Format("02-01-2006"), "", "", "In", cfg.Project,
				strings.Join(tasks, "\n"), strings.Join(notes, "\n")}
			height = math.Max(height, 15*wrappedLines(tasks, 25))
		}
		f.SetSheetRow(sheet, cell("A", r), &values)
		f.SetCellStyle(sheet, cell("A", r), cell("E", r), center)
		f.SetCellStyle(sheet, cell("F", r), cell("F", r), left)
		f.SetCellStyle(sheet, cell("G", r), cell("G", r), taskStyle)
		f.SetCellStyle(sheet, cell("H", r), cell("H", r), center)
		f.SetRowHeight(sheet, r, height)
		r++
	}
	last := r - 1

	// Summary
	f.MergeCell(sheet, cell("A", r), cell("B", r))
	f.SetCellValue(sheet, cell("A", r), "Summary :")
	f.SetCellStyle(sheet, cell("A", r), cell("B", r), labelStyle)
	for i, s := range []struct {
		label string
		value interface{}
	}{
		{"Total Working Days", fmt.Sprintf(`COUNTIF(E14:E%d,"In")`, last)},
		{"Total Non Working Days", fmt.Sprintf(`COUNTIF(E14:E%d,"Off")`, last)},
		{"Total Leave", 0},
		{"Total Overtime", 0},
	} {
		sr := r + i
		f.MergeCell(sheet, cell("C", sr), cell("E", sr))
		f.SetCellValue(sheet, cell("C", sr), s.label)
		if formula, ok := s.value.(string); ok {
			f.SetCellFormula(sheet, cell("F", sr), formula)
		} else {
			f.SetCellValue(sheet, cell("F", sr), s.value)
		}
		f.SetCellStyle(sheet, cell("C", sr), cell("F", sr), sumStyle)
	}

	// Signatures
	sign := r + 5
	f.MergeCell(sheet, cell("A", sign), cell("D", sign))
	f.SetCellValue(sheet, cell("A", sign), "Employee,")
	f.MergeCell(sheet, cell("F", sign), cell("H", sign))
	f.SetCellValue(sheet, cell("F", sign), "PIC,")
	addImage(f, sheet, cell("B", sign+1), filepath.Join(cfg.AssetsDir, "signature.png"), 180, 18, 0)
	names := sign + 5
	f.MergeCell(sheet, cell("A", names), cell("D", names))
	f.SetCellValue(sheet, cell("A", names), "("+cfg.Name+")")
	f.MergeCell(sheet, cell("F", names), cell("H", names))
	f.SetCellValue(sheet, cell("F", names), "(                              )")
	f.SetCellStyle(sheet, cell("A", sign), cell("H", names), signStyle)

	size, portrait := 9, "portrait" // A4
	f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Size: &size, Orientation: &portrait})

	name := fmt.Sprintf("Timesheet_%s_ %s - %s.xlsx", cfg.Name, indoDate(start), indoDate(end))
	return f, name, nil
}

// wrappedLines estimates how many lines lines take in a column that fits
// about width characters.
func wrappedLines(lines []string, width int) float64 {
	n := 0
	for _, l := range lines {
		n += (len([]rune(l)) + width - 1) / width
	}
	return float64(n)
}

// addImage places an image scaled to width pixels. Missing files are
// skipped so the logo and signature stay optional.
func addImage(f *excelize.File, sheet, at, path string, width float64, offX, offY int) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	img, _, err := image.DecodeConfig(file)
	file.Close()
	if err != nil || img.Width == 0 {
		log.Printf("gambar %s tidak bisa dibaca: %v", path, err)
		return
	}
	scale := width / float64(img.Width)
	if err := f.AddPicture(sheet, at, path, &excelize.GraphicOptions{
		ScaleX: scale, ScaleY: scale, OffsetX: offX, OffsetY: offY, Positioning: "oneCell",
	}); err != nil {
		log.Print(err)
	}
}
