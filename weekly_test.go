package main

import (
	"encoding/json"
	"testing"
)

func mustIssue(t *testing.T, js string) Issue {
	t.Helper()
	var is Issue
	if err := json.Unmarshal([]byte(js), &is); err != nil {
		t.Fatal(err)
	}
	return is
}

func TestWeeklyStatus(t *testing.T) {
	sub := func(typ, cat string) string {
		return `{"fields":{"issuetype":{"name":"` + typ + `"},"status":{"statusCategory":{"key":"` + cat + `"}}}}`
	}
	parent := func(qc, tr string) Issue {
		return mustIssue(t, `{"key":"P-1","fields":{"subtasks":[`+sub("Dev", "done")+`,`+sub("QC", qc)+`,`+sub("TR", tr)+`]}}`)
	}
	devDone := mustIssue(t, `{"key":"P-2","fields":{"parent":{"key":"P-1"},"issuetype":{"name":"Dev"},"status":{"name":"Done","statusCategory":{"key":"done"}}}}`)
	orphanDev := mustIssue(t, `{"key":"X-1","fields":{"issuetype":{"name":"Dev"},"status":{"name":"Done","statusCategory":{"key":"done"}}}}`)
	bugDone := mustIssue(t, `{"key":"B-1","fields":{"issuetype":{"name":"Bug"},"status":{"name":"Done","statusCategory":{"key":"done"}}}}`)
	devTodo := mustIssue(t, `{"key":"D-1","fields":{"issuetype":{"name":"Dev"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	devWip := mustIssue(t, `{"key":"D-2","fields":{"issuetype":{"name":"Dev"},"status":{"name":"In Progress","statusCategory":{"key":"indeterminate"}}}}`)

	tests := []struct {
		name    string
		is      Issue
		parents map[string]Issue
		want    interface{}
	}{
		{"dev done, QC pending", devDone, map[string]Issue{"P-1": parent("new", "new")}, "QC"},
		{"dev done, QC in progress", devDone, map[string]Issue{"P-1": parent("indeterminate", "new")}, "QC"},
		{"dev+QC done, TR pending", devDone, map[string]Issue{"P-1": parent("done", "new")}, "TR"},
		{"all done", devDone, map[string]Issue{"P-1": parent("done", "done")}, 1},
		{"dev done, no parent", orphanDev, nil, "QC"},
		{"bug done", bugDone, nil, 1},
		{"to do", devTodo, nil, 0},
		{"in progress", devWip, nil, "In Progress"},
	}
	for _, tt := range tests {
		if got := weeklyStatus(tt.is, tt.parents); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWeeklyNote(t *testing.T) {
	dev := mustIssue(t, `{"fields":{"issuetype":{"name":"Dev"}}}`)
	bug := mustIssue(t, `{"fields":{"issuetype":{"name":"Bug"}}}`)
	tests := []struct {
		is     Issue
		status interface{}
		want   string
	}{
		{dev, "QC", "DONE - Sudah submit ke QC, menunggu hasil pengujian"},
		{dev, "TR", "QC selesai, menunggu TR"},
		{dev, 1, "DONE - Selesai"},
		{bug, 1, "DONE - Bug sudah diperbaiki"},
		{dev, 0, "Rencana development minggu depan"},
		{bug, 0, "Rencana perbaikan bug minggu depan"},
		{dev, "In Progress", "DEV - Sedang dikerjakan"},
		{bug, "In Progress", "DEV - Sedang perbaikan bug"},
		{dev, "Code Review", "DEV - Code Review"},
	}
	for _, tt := range tests {
		if got := weeklyNote(tt.is, tt.status); got != tt.want {
			t.Errorf("%s %v: got %q, want %q", tt.is.Fields.IssueType.Name, tt.status, got, tt.want)
		}
	}
}

func TestHasWord(t *testing.T) {
	for name, want := range map[string]bool{"TR": true, "Sub-task TR": true, "Story": false, "QC": false} {
		if got := hasWord(name, "tr"); got != want {
			t.Errorf("hasWord(%q, tr) = %v, want %v", name, got, want)
		}
	}
}
