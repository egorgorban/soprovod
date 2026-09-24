package hh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func testServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Errorf("expected User-Agent header to be set")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_GetVacancy(t *testing.T) {
	fixture, err := os.ReadFile("testdata/vacancy.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	srv := testServer(t, http.StatusOK, fixture)
	c := NewClient(nil, srv.URL, "")

	v, err := c.GetVacancy(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("GetVacancy: %v", err)
	}

	if v.ID != "123456789" {
		t.Errorf("ID = %q, want %q", v.ID, "123456789")
	}
	if v.Title != "Backend-разработчик (Go)" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Company != "ООО Технологии Будущего" {
		t.Errorf("Company = %q", v.Company)
	}
	if v.URL != "https://hh.ru/vacancy/123456789" {
		t.Errorf("URL = %q", v.URL)
	}
	if want := "от 200 000 до 300 000 RUR"; v.Salary != want {
		t.Errorf("Salary = %q, want %q", v.Salary, want)
	}
	if len(v.KeySkills) != 5 || v.KeySkills[0] != "Go" {
		t.Errorf("KeySkills = %v", v.KeySkills)
	}
	if !strings.Contains(v.Description, "Разработка и поддержка микросервисов на Go") {
		t.Errorf("Description missing expected content: %q", v.Description)
	}
	if !strings.Contains(v.Description, "- Разработка и поддержка микросервисов на Go") {
		t.Errorf("Description should render list items with '- ' prefix: %q", v.Description)
	}
	if strings.Contains(v.Description, "<p>") || strings.Contains(v.Description, "&nbsp;") {
		t.Errorf("Description should not contain raw HTML/entities: %q", v.Description)
	}
	if len(v.Raw) == 0 {
		t.Errorf("Raw should not be empty")
	}
}

func TestClient_GetVacancy_NotFound(t *testing.T) {
	srv := testServer(t, http.StatusNotFound, []byte(`{"errors":[{"type":"not_found"}]}`))
	c := NewClient(nil, srv.URL, "")

	_, err := c.GetVacancy(context.Background(), "0")
	if err != ErrNotFound {
		t.Fatalf("GetVacancy error = %v, want ErrNotFound", err)
	}
}

func TestClient_GetVacancy_ServerError(t *testing.T) {
	srv := testServer(t, http.StatusInternalServerError, []byte(`oops`))
	c := NewClient(nil, srv.URL, "")

	_, err := c.GetVacancy(context.Background(), "1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatSalary(t *testing.T) {
	tv := true
	tests := []struct {
		name string
		s    *hhSalary
		want string
	}{
		{"nil", nil, ""},
		{"empty", &hhSalary{}, ""},
		{"from and to", &hhSalary{From: p(200000), To: p(300000), Currency: "RUR"}, "от 200 000 до 300 000 RUR"},
		{"from only", &hhSalary{From: p(150000), Currency: "RUR"}, "от 150 000 RUR"},
		{"to only", &hhSalary{To: p(100000), Currency: "USD"}, "до 100 000 USD"},
		{"gross", &hhSalary{From: p(1000), Currency: "EUR", Gross: &tv}, "от 1 000 EUR до вычета налогов"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSalary(tt.s)
			if got != tt.want {
				t.Errorf("formatSalary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func p(n int64) *int64 { return &n }
