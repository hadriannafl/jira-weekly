package main

import (
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

//go:embed index.html
var indexHTML string

var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"day":      day,
	"ago":      ago,
	"lower":    strings.ToLower,
	"doneDate": func(is Issue) string { return day(doneTime(is)) },
}).Parse(indexHTML))

var sectionIDs = map[string]string{catTodo: "todo", catProgress: "progress", catDone: "done", catBug: "bug"}

type section struct {
	ID     string
	Name   string
	Issues []Issue
}

type month struct {
	Label  string
	Issues []Issue
}

func serveWeb(addr string, client *Client, jql string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		week := weekParam(r)
		rep, err := fetchReport(client, week, jql)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		var sections []section
		for _, c := range categories {
			sections = append(sections, section{ID: sectionIDs[c], Name: c, Issues: rep.Groups[c]})
		}
		donePct := 0
		if rep.Total > 0 {
			donePct = len(rep.Groups[catDone]) * 100 / rep.Total
		}
		_, weekNum := rep.Start.ISOWeek()
		data := pageData(client, "weekly")
		data["Sections"] = sections
		data["Total"] = rep.Total
		data["Done"] = len(rep.Groups[catDone])
		data["DonePct"] = donePct
		data["WeekNum"] = weekNum
		data["WeekName"] = weekName(week)
		data["Label"] = rep.Start.Format("02 Jan") + " – " + rep.End.AddDate(0, 0, -1).Format("02 Jan 2006")
		data["Week"] = week
		data["Prev"] = week - 1
		data["Next"] = week + 1
		render(w, data)
	})

	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		if days <= 0 {
			days = 90
		}
		issues, err := client.Search(fmt.Sprintf(
			`(assignee = currentUser() OR assignee was currentUser()) AND statusCategory = Done AND updated >= -%dd`, days), "")
		if err == nil {
			err = setFrom(client, issues)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		sort.Slice(issues, func(i, j int) bool { return doneTime(issues[i]) > doneTime(issues[j]) })

		var months []month
		for _, is := range issues {
			label := doneTime(is)
			if t, err := time.Parse(jiraTime, label); err == nil {
				t = t.In(time.Local)
				label = indoMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())
			}
			if len(months) == 0 || months[len(months)-1].Label != label {
				months = append(months, month{Label: label})
			}
			months[len(months)-1].Issues = append(months[len(months)-1].Issues, is)
		}

		data := pageData(client, "history")
		data["Months"] = months
		data["Total"] = len(issues)
		data["Days"] = days
		data["Ranges"] = []int{30, 90, 365}
		render(w, data)
	})

	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		f, name, err := buildWeekly(client, weekParam(r))
		sendExcel(w, f, name, err)
	})

	mux.HandleFunc("/timesheet", func(w http.ResponseWriter, r *http.Request) {
		f, name, err := buildTimesheet(client, loadTimesheetConfig(client), r.URL.Query().Get("period"))
		sendExcel(w, f, name, err)
	})

	log.Printf("Dashboard web jalan di %s", addr)
	return http.ListenAndServe(addr, basicAuth(mux))
}

func pageData(client *Client, page string) map[string]any {
	initials := ""
	for _, w := range strings.Fields(client.DisplayName) {
		if len(initials) < 2 {
			initials += strings.ToUpper(string([]rune(w)[0]))
		}
	}
	return map[string]any{
		"Page":     page,
		"BaseURL":  client.BaseURL,
		"Name":     client.DisplayName,
		"Initials": initials,
		"Fetched":  time.Now().Format("15:04"),
		"ZoomURL":   os.Getenv("ZOOM_URL"),
		"Reminders": reminders(time.Now()),
	}
}

func render(w http.ResponseWriter, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTmpl.Execute(w, data); err != nil {
		log.Print(err)
	}
}

func weekName(week int) string {
	switch week {
	case 0:
		return "Minggu ini"
	case -1:
		return "Minggu lalu"
	}
	if week < 0 {
		return fmt.Sprintf("%d minggu lalu", -week)
	}
	return fmt.Sprintf("%d minggu lagi", week)
}

// doneTime is when the issue reached Done, falling back to its last update.
func doneTime(is Issue) string {
	if is.Fields.StatusCategoryChangeDate != "" {
		return is.Fields.StatusCategoryChangeDate
	}
	return is.Fields.Updated
}

func sendExcel(w http.ResponseWriter, f *excelize.File, name string, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	if err := f.Write(w); err != nil {
		log.Print(err)
	}
}

// day trims Jira timestamps like 2026-09-22T10:15:30.000+0700 to the date.
func day(s string) string {
	if t, err := time.Parse(jiraTime, s); err == nil {
		return t.In(time.Local).Format("02 Jan 2006")
	}
	return s
}

// ago turns a Jira timestamp into "3 jam lalu" style text.
func ago(s string) string {
	t, err := time.Parse(jiraTime, s)
	if err != nil {
		return s
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam lalu", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d hari lalu", int(d.Hours()/24))
	}
	return t.In(time.Local).Format("02 Jan 2006")
}

func weekParam(r *http.Request) int {
	week, _ := strconv.Atoi(r.URL.Query().Get("week"))
	return week
}

// basicAuth protects the dashboard when WEB_PASSWORD is set.
func basicAuth(next http.Handler) http.Handler {
	user, pass := os.Getenv("WEB_USER"), os.Getenv("WEB_PASSWORD")
	if pass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="jira-weekly"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
