package storage

import (
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned by Repo methods when the requested row does not exist.
var ErrNotFound = errors.New("storage: not found")

// Vacancy source values.
const (
	VacancySourceHH     = "hh"
	VacancySourceManual = "manual"
)

// Vacancy mirrors the vacancies table. HHID and URL are empty strings when
// absent (manual vacancies), mirroring the SQL NULL -> "" convention used
// elsewhere in this package (see nullIfEmpty).
type Vacancy struct {
	ID          int64           `json:"id"`
	Source      string          `json:"source"`
	HHID        string          `json:"hh_id"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Company     string          `json:"company"`
	Salary      string          `json:"salary"`
	Description string          `json:"description"`
	KeySkills   []string        `json:"key_skills"`
	Raw         json.RawMessage `json:"raw"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Application mirrors the applications table.
type Application struct {
	ID            int64           `json:"id"`
	VacancyID     int64           `json:"vacancy_id"`
	FilterPassed  bool            `json:"filter_passed"`
	FilterReason  string          `json:"filter_reason"`
	Status        string          `json:"status"`
	GeneratedText string          `json:"generated_text"`
	EditedText    string          `json:"edited_text"`
	LLMOutput     json.RawMessage `json:"llm_output"`
	Model         string          `json:"model"`
	Error         string          `json:"error"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Application status values.
const (
	StatusGenerated   = "generated"
	StatusFilteredOut = "filtered_out"
	StatusFailed      = "failed"
)

// ApplicationListItem is a row of the applications list view: application
// fields joined with vacancy title/company plus a short letter preview.
type ApplicationListItem struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Company   string    `json:"company"`
	Status    string    `json:"status"`
	Preview   string    `json:"preview"`
	CreatedAt time.Time `json:"created_at"`
}

// ApplicationWithVacancy bundles an application with its parent vacancy, as
// returned by GetApplication.
type ApplicationWithVacancy struct {
	Application Application `json:"application"`
	Vacancy     Vacancy     `json:"vacancy"`
}
