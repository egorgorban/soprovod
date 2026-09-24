// Package hhparser fetches hh.ru vacancies by scraping the public vacancy
// HTML page (as opposed to internal/hh, which uses the api.hh.ru JSON API).
// It implements the same GetVacancy(ctx, id) (*hh.Vacancy, error) shape as
// hh.Client so it can be used as a drop-in pipeline.VacancyFetcher.
package hhparser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/egorgorban/soprovod/backend/internal/hh"
)

const (
	defaultBaseURL = "https://hh.ru"
	defaultTimeout = 10 * time.Second

	// defaultUserAgent mimics a recent desktop Chrome build. hh.ru serves a
	// different (JS-heavy, captcha-prone) page to obvious bot user agents.
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
)

// whitespaceRe matches runs of whitespace, used to collapse hh.ru's
// pretty-printed source HTML before text extraction.
var whitespaceRe = regexp.MustCompile(`\s+`)

// ErrBlocked is returned when hh.ru refuses the request with a 403/429 or
// serves a captcha/anti-bot page instead of the vacancy.
var ErrBlocked = errors.New("hhparser: blocked by hh.ru (captcha or rate limit)")

// ErrParse is returned when the vacancy page was fetched successfully but no
// usable vacancy data (in particular, a description) could be extracted
// from it.
var ErrParse = errors.New("hhparser: failed to parse vacancy page")

// Client fetches and parses vacancies from the hh.ru vacancy HTML page.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. If httpClient is nil, a client with a 10s
// timeout is used. If baseURL is empty, "https://hh.ru" is used.
func NewClient(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// GetVacancy fetches and parses the hh.ru vacancy page for id.
func (c *Client) GetVacancy(ctx context.Context, id string) (*hh.Vacancy, error) {
	pageURL := fmt.Sprintf("%s/vacancy/%s", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hhparser: build request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hhparser: request vacancy %s: %w", id, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hhparser: read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, hh.ErrNotFound
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w: status %d", ErrBlocked, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("hhparser: unexpected status %d for vacancy %s", resp.StatusCode, id)
	}

	if looksBlocked(body) {
		return nil, ErrBlocked
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("hhparser: parse html: %w", err)
	}

	extracted := extract(doc)
	if strings.TrimSpace(extracted.Description) == "" {
		return nil, fmt.Errorf("%w: empty description for vacancy %s", ErrParse, id)
	}

	url := pageURL
	if canonical, ok := doc.Find(`link[rel="canonical"]`).Attr("href"); ok && canonical != "" {
		url = canonical
	}

	raw, err := json.Marshal(rawVacancy{
		Source:      "html",
		Title:       extracted.Title,
		Company:     extracted.Company,
		Salary:      extracted.Salary,
		Description: extracted.Description,
		KeySkills:   extracted.KeySkills,
	})
	if err != nil {
		return nil, fmt.Errorf("hhparser: marshal raw: %w", err)
	}

	return &hh.Vacancy{
		ID:          id,
		URL:         url,
		Title:       extracted.Title,
		Company:     extracted.Company,
		Salary:      extracted.Salary,
		Description: extracted.Description,
		KeySkills:   extracted.KeySkills,
		Raw:         json.RawMessage(raw),
	}, nil
}

// rawVacancy is what we store in hh.Vacancy.Raw for HTML-sourced vacancies.
// It intentionally mirrors only the fields we actually extracted, plus a
// "source" marker so consumers of the jsonb column can tell HTML- and
// API-sourced rows apart.
type rawVacancy struct {
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Salary      string   `json:"salary,omitempty"`
	Description string   `json:"description"`
	KeySkills   []string `json:"key_skills,omitempty"`
}

// looksBlocked reports whether body looks like a hh.ru/Yandex SmartCaptcha
// anti-bot page rather than a real vacancy page.
func looksBlocked(body []byte) bool {
	// Cheap, case-insensitive substring scan over a lowered copy; these
	// pages are small enough that this is not a performance concern.
	s := strings.ToLower(string(body))
	markers := []string{
		"showcaptcha",
		"smartcaptcha",
		"<title>антиробот",
		"<title>подтвердите, что запрос сделан человеком",
		"captcha-container",
		`action="/captcha`,
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// jobPosting mirrors the subset of schema.org JobPosting JSON-LD hh.ru
// embeds on vacancy pages.
type jobPosting struct {
	Type               string            `json:"@type"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	HiringOrganization *jobPostingOrg    `json:"hiringOrganization"`
	BaseSalary         *jobPostingSalary `json:"baseSalary"`
}

type jobPostingOrg struct {
	Name string `json:"name"`
}

type jobPostingSalary struct {
	Currency string                 `json:"currency"`
	Value    *jobPostingSalaryValue `json:"value"`
}

type jobPostingSalaryValue struct {
	MinValue json.Number `json:"minValue"`
	MaxValue json.Number `json:"maxValue"`
	Value    json.Number `json:"value"`
	UnitText string      `json:"unitText"`
}

type extracted struct {
	Title       string
	Company     string
	Salary      string
	Description string
	KeySkills   []string
}

// extract pulls vacancy data out of doc, starting from any JSON-LD
// JobPosting block and layering DOM (data-qa) values on top: DOM values win
// whenever present, JSON-LD is only used to fill gaps (or, for salary, as
// the sole source since hh.ru does not expose salary via data-qa).
func extract(doc *goquery.Document) extracted {
	var jp jobPosting
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var candidate jobPosting
		if err := json.Unmarshal([]byte(s.Text()), &candidate); err != nil {
			return true // keep looking
		}
		if candidate.Type == "JobPosting" {
			jp = candidate
			return false // stop, found it
		}
		return true
	})

	out := extracted{KeySkills: []string{}}
	out.Title = jp.Title
	if jp.HiringOrganization != nil {
		out.Company = jp.HiringOrganization.Name
	}
	if jp.Description != "" {
		out.Description = hh.HTMLToText(jp.Description)
	}
	out.Salary = formatJSONLDSalary(jp.BaseSalary)

	if v := collapseSpaces(doc.Find(`[data-qa="vacancy-title"]`).First().Text()); v != "" {
		out.Title = v
	}
	if v := collapseSpaces(doc.Find(`[data-qa="vacancy-company-name"]`).First().Text()); v != "" {
		out.Company = v
	}
	if sel := doc.Find(`[data-qa^="vacancy-salary"]`).First(); sel.Length() > 0 {
		if v := strings.TrimSpace(sel.Text()); v != "" {
			out.Salary = collapseSpaces(v)
		}
	}
	if sel := doc.Find(`[data-qa="vacancy-description"]`).First(); sel.Length() > 0 {
		if descHTML, err := sel.Html(); err == nil {
			// hh.ru serves the vacancy page pretty-printed: text nodes
			// contain source-formatting newlines/indentation that are not
			// meaningful whitespace. Collapse it before handing off to
			// hh.HTMLToText, which otherwise treats every literal newline
			// as an intentional line break.
			if v := hh.HTMLToText(whitespaceRe.ReplaceAllString(descHTML, " ")); v != "" {
				out.Description = v
			}
		}
	}

	doc.Find(`[data-qa="skills-element"]`).Each(func(_ int, s *goquery.Selection) {
		if v := strings.TrimSpace(s.Text()); v != "" {
			out.KeySkills = append(out.KeySkills, v)
		}
	})

	return out
}

// formatJSONLDSalary renders a JSON-LD baseSalary block into a
// human-readable Russian string similar to hh.Client's formatSalary, e.g.
// "от 200 000 до 300 000 RUR".
func formatJSONLDSalary(bs *jobPostingSalary) string {
	if bs == nil || bs.Value == nil {
		return ""
	}
	v := bs.Value

	min, minOK := parseJSONNumber(v.MinValue)
	max, maxOK := parseJSONNumber(v.MaxValue)
	if !minOK && !maxOK {
		if single, ok := parseJSONNumber(v.Value); ok {
			min, minOK = single, true
		}
	}
	if !minOK && !maxOK {
		return ""
	}

	var b strings.Builder
	switch {
	case minOK && maxOK:
		b.WriteString("от ")
		b.WriteString(formatThousands(min))
		b.WriteString(" до ")
		b.WriteString(formatThousands(max))
	case minOK:
		b.WriteString("от ")
		b.WriteString(formatThousands(min))
	case maxOK:
		b.WriteString("до ")
		b.WriteString(formatThousands(max))
	}
	if bs.Currency != "" {
		b.WriteString(" ")
		b.WriteString(bs.Currency)
	}
	return b.String()
}

func parseJSONNumber(n json.Number) (int64, bool) {
	if n == "" {
		return 0, false
	}
	v, err := n.Int64()
	if err != nil {
		f, ferr := strconv.ParseFloat(n.String(), 64)
		if ferr != nil {
			return 0, false
		}
		return int64(f), true
	}
	return v, true
}

// formatThousands renders n with a space as the thousands separator, e.g.
// 200000 -> "200 000".
func formatThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
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

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
