package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL string
	Email   string
	Token   string
	HTTP    *http.Client

	// Filled by CheckAuth.
	AccountID   string
	DisplayName string
}

type Issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Reporter *struct {
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent"`
		Created string `json:"created"`
		Updated string `json:"updated"`
		DueDate string `json:"duedate"`
		// When the issue last moved between To Do / In Progress / Done.
		StatusCategoryChangeDate string `json:"statuscategorychangedate"`
	} `json:"fields"`
	// From is who handed the ticket to you, filled by setFrom.
	From string `json:"-"`

	Changelog struct {
		Histories []struct {
			Author struct {
				AccountID string `json:"accountId"`
			} `json:"author"`
			Created string `json:"created"`
		} `json:"histories"`
	} `json:"changelog"`
}

type searchPage struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
	IsLast        bool    `json:"isLast"`
}

// CheckAuth verifies the credentials. Needed because Jira answers a search
// with bad credentials as an anonymous user (empty result, no error).
func (c *Client) CheckAuth() error {
	req, err := http.NewRequest("GET", c.BaseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Email, c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login Jira gagal (%s): cek JIRA_EMAIL dan JIRA_API_TOKEN di .env", resp.Status)
	}
	var me struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return err
	}
	c.AccountID, c.DisplayName = me.AccountID, me.DisplayName
	return nil
}

// Search fetches every issue matching jql, following pagination. expand may
// be empty or e.g. "changelog".
func (c *Client) Search(jql, expand string) ([]Issue, error) {
	var all []Issue
	token := ""
	for {
		page, err := c.searchPage(jql, expand, token)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Issues...)
		if page.IsLast || page.NextPageToken == "" {
			return all, nil
		}
		token = page.NextPageToken
	}
}

func (c *Client) searchPage(jql, expand, token string) (*searchPage, error) {
	q := url.Values{}
	q.Set("jql", jql)
	q.Set("fields", "summary,status,issuetype,priority,project,reporter,assignee,parent,created,updated,duedate,statuscategorychangedate")
	q.Set("maxResults", "100")
	if expand != "" {
		q.Set("expand", expand)
	}
	if token != "" {
		q.Set("nextPageToken", token)
	}
	req, err := http.NewRequest("GET", c.BaseURL+"/rest/api/3/search/jql?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("jira: %s: %s", resp.Status, body)
	}

	var page searchPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}
	return &page, nil
}

const (
	catTodo     = "To Do"
	catProgress = "In Progress"
	catDone     = "Done"
	catBug      = "Bug"
)

var categories = []string{catTodo, catProgress, catDone, catBug}

// category puts bugs in their own bucket; everything else is grouped by
// Jira's status category so custom workflow status names still land right.
func category(is Issue) string {
	if strings.EqualFold(is.Fields.IssueType.Name, "Bug") {
		return catBug
	}
	switch is.Fields.Status.StatusCategory.Key {
	case "indeterminate":
		return catProgress
	case "done":
		return catDone
	}
	return catTodo
}
