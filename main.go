package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	week := flag.Int("week", 0, "minggu yang dilaporkan: 0 = minggu ini, -1 = minggu lalu")
	outDir := flag.String("out", ".", "folder tujuan file Excel")
	customJQL := flag.String("jql", "", "JQL sendiri, menggantikan JQL default")
	interval := flag.Duration("interval", 0, "mode monitoring, cek ulang tiap interval (contoh: 15m)")
	web := flag.String("web", "", "jalankan dashboard web di alamat ini (contoh: :8080)")
	timesheet := flag.Bool("timesheet", false, "buat file timesheet bulanan (periode 22 - 21)")
	period := flag.String("period", "", "periode timesheet YYYY-MM, bulan tanggal 21-nya (default: periode berjalan)")
	flag.Parse()

	loadEnv(".env")
	client := &Client{
		BaseURL: strings.TrimRight(os.Getenv("JIRA_BASE_URL"), "/"),
		Email:   os.Getenv("JIRA_EMAIL"),
		Token:   os.Getenv("JIRA_API_TOKEN"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
	if client.BaseURL == "" || client.Email == "" || client.Token == "" {
		log.Fatal("JIRA_BASE_URL, JIRA_EMAIL dan JIRA_API_TOKEN wajib diisi (lihat .env.example)")
	}
	if err := client.CheckAuth(); err != nil {
		log.Fatal(err)
	}

	if *timesheet {
		f, name, err := buildTimesheet(client, loadTimesheetConfig(client), *period)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		path := filepath.Join(*outDir, name)
		if err := f.SaveAs(path); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Timesheet:", path)
		return
	}
	if *web != "" {
		if *interval > 0 {
			go monitor(client, *week, *outDir, *customJQL, *interval)
		}
		log.Fatal(serveWeb(*web, client, *customJQL))
	}
	if *interval > 0 {
		monitor(client, *week, *outDir, *customJQL, *interval)
	}
	if err := run(client, *week, *outDir, *customJQL); err != nil {
		log.Fatal(err)
	}
}

func monitor(client *Client, week int, outDir, jql string, interval time.Duration) {
	for {
		if err := run(client, week, outDir, jql); err != nil {
			log.Print(err)
		}
		time.Sleep(interval)
	}
}

type Report struct {
	Start, End time.Time
	Groups     map[string][]Issue
	Total      int
}

func fetchReport(client *Client, week int, jql string) (*Report, error) {
	start, end := weekRange(time.Now(), week)
	if jql == "" {
		// Open tasks, plus anything touched during the week (covers tasks
		// finished that week).
		jql = fmt.Sprintf(`assignee = currentUser() AND (statusCategory != Done OR (updated >= "%s" AND updated < "%s")) ORDER BY updated DESC`,
			start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	issues, err := client.Search(jql, "")
	if err != nil {
		return nil, err
	}
	if err := setFrom(client, issues); err != nil {
		return nil, err
	}

	groups := map[string][]Issue{}
	for _, is := range issues {
		c := category(is)
		groups[c] = append(groups[c], is)
	}
	return &Report{Start: start, End: end, Groups: groups, Total: len(issues)}, nil
}

func run(client *Client, week int, outDir, jql string) error {
	rep, err := fetchReport(client, week, jql)
	if err != nil {
		return err
	}

	f, name, err := buildWeekly(client, week)
	if err != nil {
		return err
	}
	defer f.Close()
	path := filepath.Join(outDir, name)
	if err := f.SaveAs(path); err != nil {
		return err
	}

	fmt.Printf("[%s] Minggu %s - %s, %d issue\n", time.Now().Format("15:04"),
		rep.Start.Format("02 Jan"), rep.End.AddDate(0, 0, -1).Format("02 Jan 2006"), rep.Total)
	for _, c := range categories {
		fmt.Printf("  %-12s %d\n", c, len(rep.Groups[c]))
	}
	fmt.Println("  Excel:", path)
	return nil
}

// weekRange returns Monday 00:00 of the target week and the following Monday.
func weekRange(now time.Time, offset int) (time.Time, time.Time) {
	sinceMonday := (int(now.Weekday()) + 6) % 7
	start := time.Date(now.Year(), now.Month(), now.Day()-sinceMonday+7*offset, 0, 0, 0, 0, now.Location())
	return start, start.AddDate(0, 0, 7)
}

// loadEnv reads KEY=VALUE lines from path. Variables already set in the
// environment win.
func loadEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if ok && os.Getenv(k) == "" {
			os.Setenv(k, strings.Trim(strings.TrimSpace(v), `"`))
		}
	}
}
