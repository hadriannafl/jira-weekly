package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	jiraTime = "2006-01-02T15:04:05.000-0700"
	dateKey  = "2006-01-02"
)

var indoMonths = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli",
	"Agustus", "September", "Oktober", "November", "Desember"}

// indoDate formats t like "25 September 2026".
func indoDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), indoMonths[t.Month()-1], t.Year())
}

// fetchActivity returns your open issues plus every issue you are or were
// assigned to that changed since from, with changelog.
func fetchActivity(client *Client, from time.Time) ([]Issue, error) {
	jql := fmt.Sprintf(`(assignee = currentUser() AND statusCategory != Done) OR ((assignee = currentUser() OR assignee was currentUser()) AND updated >= "%s") ORDER BY key`,
		from.Format(dateKey))
	return client.Search(jql, "changelog")
}

// fetchParents returns the parent tickets of issues by key, including their
// subtasks.
func fetchParents(client *Client, issues []Issue) (map[string]Issue, error) {
	var keys []string
	seen := map[string]bool{}
	for _, is := range issues {
		if p := is.Fields.Parent; p != nil && !seen[p.Key] {
			seen[p.Key] = true
			keys = append(keys, p.Key)
		}
	}

	parents := map[string]Issue{}
	for i := 0; i < len(keys); i += 100 {
		batch := keys[i:min(i+100, len(keys))]
		found, err := client.Search("key in ("+strings.Join(batch, ",")+")", "")
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			parents[p.Key] = p
		}
	}
	return parents, nil
}

// setFrom fills Issue.From with the assignee of the parent ticket (the
// person who split it into subtasks for you), falling back to the reporter.
func setFrom(client *Client, issues []Issue) error {
	parents, err := fetchParents(client, issues)
	if err != nil {
		return err
	}
	for i := range issues {
		f := &issues[i].Fields
		if p, ok := parents[parentKey(issues[i])]; ok && p.Fields.Assignee != nil {
			issues[i].From = p.Fields.Assignee.DisplayName
		} else if f.Reporter != nil {
			issues[i].From = f.Reporter.DisplayName
		}
	}
	return nil
}

func parentKey(is Issue) string {
	if is.Fields.Parent == nil {
		return ""
	}
	return is.Fields.Parent.Key
}

// activityDays returns the local dates (YYYY-MM-DD) on which accountID
// changed the issue, according to its changelog.
func activityDays(is Issue, accountID string) map[string]bool {
	days := map[string]bool{}
	for _, h := range is.Changelog.Histories {
		if h.Author.AccountID != accountID {
			continue
		}
		if t, err := time.Parse(jiraTime, h.Created); err == nil {
			days[t.In(time.Local).Format(dateKey)] = true
		}
	}
	return days
}

// activeBetween reports whether any day in [from, to) is in days.
func activeBetween(days map[string]bool, from, to time.Time) bool {
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		if days[d.Format(dateKey)] {
			return true
		}
	}
	return false
}
