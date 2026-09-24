package hhparser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/egorgorban/soprovod/backend/internal/hh"
)

// fixtureServer serves the given file's contents at every path, with the
// given status code, and asserts a browser-like User-Agent was sent.
func fixtureServer(t *testing.T, status int, fixturePath string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "Chrome") {
			t.Errorf("expected a Chrome-like User-Agent, got %q", ua)
		}
		if r.Header.Get("Accept-Language") == "" {
			t.Errorf("expected Accept-Language header to be set")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_GetVacancy_RealFixture1(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/vacancy_real1.html")
	c := NewClient(nil, srv.URL)

	v, err := c.GetVacancy(context.Background(), "111")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}

	if v.Title != "Data Engineer (Big data middle)" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Company != "Emphasoft" {
		t.Errorf("Company = %q", v.Company)
	}
	if !strings.Contains(v.Description, "Emphasoft занимается") {
		t.Errorf("Description missing expected content: %q", v.Description)
	}
	if strings.Contains(v.Description, "<p>") || strings.Contains(v.Description, "&nbsp;") {
		t.Errorf("Description should not contain raw HTML/entities: %q", v.Description)
	}
	// This real fixture has no key skills section on the page.
	if len(v.KeySkills) != 0 {
		t.Errorf("KeySkills = %v, want none", v.KeySkills)
	}
	if len(v.Raw) == 0 {
		t.Errorf("Raw should not be empty")
	}
}

func TestClient_GetVacancy_RealFixture2(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/vacancy_real2.html")
	c := NewClient(nil, srv.URL)

	v, err := c.GetVacancy(context.Background(), "222")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}

	if v.Title != "Senior Python разработчик" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Company != "Литрес" {
		t.Errorf("Company = %q", v.Company)
	}
	if len(v.KeySkills) == 0 {
		t.Errorf("expected key skills to be extracted, got none")
	}
	found := false
	for _, s := range v.KeySkills {
		if s == "Python" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected KeySkills to contain %q, got %v", "Python", v.KeySkills)
	}
	if v.Description == "" {
		t.Errorf("Description should not be empty")
	}
}

func TestClient_GetVacancy_RealFixture3(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/vacancy_real3.html")
	c := NewClient(nil, srv.URL)

	v, err := c.GetVacancy(context.Background(), "333")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}

	if v.Title != "Python Developer (AD Robot)" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Company == "" {
		t.Errorf("Company should not be empty")
	}
	if len(v.KeySkills) < 2 {
		t.Errorf("expected several key skills, got %v", v.KeySkills)
	}
	if !strings.Contains(v.Description, "Eto Legko") {
		t.Errorf("Description missing expected content: %q", v.Description)
	}
}

func TestClient_GetVacancy_DOMOnly(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/vacancy_dom_only_synthetic.html")
	c := NewClient(nil, srv.URL)

	v, err := c.GetVacancy(context.Background(), "444000222")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}
	if v.Title != "Frontend-разработчик" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Company != "ООО Вебстудия" {
		t.Errorf("Company = %q", v.Company)
	}
	if !strings.Contains(v.Description, "Верстка и интеграция") {
		t.Errorf("Description = %q", v.Description)
	}
	if len(v.KeySkills) != 2 {
		t.Errorf("KeySkills = %v", v.KeySkills)
	}
	if v.URL != "https://hh.ru/vacancy/444000222" {
		t.Errorf("URL = %q, want canonical link value", v.URL)
	}
}

func TestClient_GetVacancy_Salary(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/vacancy_salary_synthetic.html")
	c := NewClient(nil, srv.URL)

	v, err := c.GetVacancy(context.Background(), "555000111")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}
	// DOM data-qa="vacancy-salary-*" wins over JSON-LD baseSalary.
	if !strings.Contains(v.Salary, "200 000") || !strings.Contains(v.Salary, "300 000") {
		t.Errorf("Salary = %q", v.Salary)
	}
}

func TestClient_GetVacancy_NotFound(t *testing.T) {
	srv := fixtureServer(t, http.StatusNotFound, "testdata/empty_description_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "999999999")
	if !errors.Is(err, hh.ErrNotFound) {
		t.Fatalf("GetVacancy error = %v, want hh.ErrNotFound", err)
	}
}

func TestClient_GetVacancy_Forbidden(t *testing.T) {
	srv := fixtureServer(t, http.StatusForbidden, "testdata/captcha_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "1")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("GetVacancy error = %v, want ErrBlocked", err)
	}
}

func TestClient_GetVacancy_TooManyRequests(t *testing.T) {
	srv := fixtureServer(t, http.StatusTooManyRequests, "testdata/captcha_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "1")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("GetVacancy error = %v, want ErrBlocked", err)
	}
}

func TestClient_GetVacancy_CaptchaPageOn200(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/captcha_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "1")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("GetVacancy error = %v, want ErrBlocked", err)
	}
}

func TestClient_GetVacancy_EmptyDescription(t *testing.T) {
	srv := fixtureServer(t, http.StatusOK, "testdata/empty_description_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "999999999")
	if !errors.Is(err, ErrParse) {
		t.Fatalf("GetVacancy error = %v, want ErrParse", err)
	}
}

func TestClient_GetVacancy_ServerError(t *testing.T) {
	srv := fixtureServer(t, http.StatusInternalServerError, "testdata/captcha_synthetic.html")
	c := NewClient(nil, srv.URL)

	_, err := c.GetVacancy(context.Background(), "1")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrBlocked) || errors.Is(err, hh.ErrNotFound) {
		t.Fatalf("expected a generic error, got %v", err)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(nil, "")
	if c.httpClient == nil {
		t.Error("expected a default http.Client")
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}
