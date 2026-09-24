package hh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://api.hh.ru"
	defaultUserAgent = "soprovod/1.0 (egorgorban239@gmail.com)"
	defaultTimeout   = 10 * time.Second
)

// ErrNotFound is returned by Client.GetVacancy when hh.ru reports the
// vacancy does not exist (HTTP 404).
var ErrNotFound = errors.New("hh: vacancy not found")

// Vacancy is a simplified, ready-to-use view of a hh.ru vacancy.
type Vacancy struct {
	ID          string
	URL         string
	Title       string
	Company     string
	Salary      string
	Description string
	KeySkills   []string
	Raw         json.RawMessage
}

// Client fetches vacancies from the hh.ru public API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// NewClient builds a Client. If httpClient is nil, a client with a 10s
// timeout is used. If baseURL is empty, "https://api.hh.ru" is used. If
// userAgent is empty, a default identifying user agent is used.
func NewClient(httpClient *http.Client, baseURL, userAgent string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		userAgent:  userAgent,
	}
}

// hhVacancy mirrors the subset of the hh.ru API vacancy JSON we care about.
type hhVacancy struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	AlternateURL string       `json:"alternate_url"`
	Salary       *hhSalary    `json:"salary"`
	SalaryRange  *hhSalary    `json:"salary_range"`
	KeySkills    []hhKeySkill `json:"key_skills"`
	Employer     *hhEmployer  `json:"employer"`
}

type hhKeySkill struct {
	Name string `json:"name"`
}

type hhEmployer struct {
	Name string `json:"name"`
}

type hhSalary struct {
	From     *int64 `json:"from"`
	To       *int64 `json:"to"`
	Currency string `json:"currency"`
	Gross    *bool  `json:"gross"`
}

// GetVacancy fetches and parses the vacancy identified by id (as returned
// by ParseVacancyID).
func (c *Client) GetVacancy(ctx context.Context, id string) (*Vacancy, error) {
	url := fmt.Sprintf("%s/vacancies/%s", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hh: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hh: request vacancy %s: %w", id, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hh: read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hh: unexpected status %d for vacancy %s", resp.StatusCode, id)
	}

	var v hhVacancy
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("hh: decode vacancy %s: %w", id, err)
	}

	skills := make([]string, 0, len(v.KeySkills))
	for _, s := range v.KeySkills {
		if s.Name != "" {
			skills = append(skills, s.Name)
		}
	}

	company := ""
	if v.Employer != nil {
		company = v.Employer.Name
	}

	salary := v.Salary
	if salary == nil {
		salary = v.SalaryRange
	}

	return &Vacancy{
		ID:          v.ID,
		URL:         v.AlternateURL,
		Title:       v.Name,
		Company:     company,
		Salary:      formatSalary(salary),
		Description: htmlToText(v.Description),
		KeySkills:   skills,
		Raw:         json.RawMessage(raw),
	}, nil
}

// formatSalary formats a hh.ru salary object into a human-readable Russian
// string, e.g. "от 200 000 до 300 000 RUR". Returns "" if s is nil or both
// From and To are nil.
func formatSalary(s *hhSalary) string {
	if s == nil || (s.From == nil && s.To == nil) {
		return ""
	}

	var b strings.Builder
	switch {
	case s.From != nil && s.To != nil:
		b.WriteString("от ")
		b.WriteString(formatThousands(*s.From))
		b.WriteString(" до ")
		b.WriteString(formatThousands(*s.To))
	case s.From != nil:
		b.WriteString("от ")
		b.WriteString(formatThousands(*s.From))
	case s.To != nil:
		b.WriteString("до ")
		b.WriteString(formatThousands(*s.To))
	}

	if s.Currency != "" {
		b.WriteString(" ")
		b.WriteString(s.Currency)
	}
	if s.Gross != nil && *s.Gross {
		b.WriteString(" до вычета налогов")
	}

	return b.String()
}

// formatThousands renders n with a space as the thousands separator, e.g.
// 200000 -> "200 000".
func formatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var groups []string
	for len(s) > 3 {
		groups = append([]string{s[len(s)-3:]}, groups...)
		s = s[:len(s)-3]
	}
	groups = append([]string{s}, groups...)
	out := strings.Join(groups, " ")
	if neg {
		out = "-" + out
	}
	return out
}
