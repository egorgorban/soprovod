package httpapi

import (
	"encoding/json"
	"time"

	"github.com/egorgorban/soprovod/backend/internal/storage"
)

// vacancyDTO mirrors frontend/src/types.ts Vacancy: salary is nullable,
// key_skills is never null.
type vacancyDTO struct {
	ID          int64     `json:"id"`
	HHID        string    `json:"hh_id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Company     string    `json:"company"`
	Salary      *string   `json:"salary"`
	Description string    `json:"description"`
	KeySkills   []string  `json:"key_skills"`
	CreatedAt   time.Time `json:"created_at"`
}

func newVacancyDTO(v storage.Vacancy) vacancyDTO {
	skills := v.KeySkills
	if skills == nil {
		skills = []string{}
	}
	return vacancyDTO{
		ID:          v.ID,
		HHID:        v.HHID,
		URL:         v.URL,
		Title:       v.Title,
		Company:     v.Company,
		Salary:      strPtrOrNil(v.Salary),
		Description: v.Description,
		KeySkills:   skills,
		CreatedAt:   v.CreatedAt,
	}
}

// applicationDTO mirrors frontend/src/types.ts Application: most text fields
// are nullable, llm_output is a JSON object or null.
type applicationDTO struct {
	ID            int64           `json:"id"`
	VacancyID     int64           `json:"vacancy_id"`
	FilterPassed  bool            `json:"filter_passed"`
	FilterReason  *string         `json:"filter_reason"`
	Status        string          `json:"status"`
	GeneratedText *string         `json:"generated_text"`
	EditedText    *string         `json:"edited_text"`
	LLMOutput     json.RawMessage `json:"llm_output"`
	Model         *string         `json:"model"`
	Error         *string         `json:"error"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func newApplicationDTO(a storage.Application) applicationDTO {
	llm := json.RawMessage("null")
	if len(a.LLMOutput) > 0 && string(a.LLMOutput) != "null" {
		llm = a.LLMOutput
	}
	return applicationDTO{
		ID:            a.ID,
		VacancyID:     a.VacancyID,
		FilterPassed:  a.FilterPassed,
		FilterReason:  strPtrOrNil(a.FilterReason),
		Status:        a.Status,
		GeneratedText: strPtrOrNil(a.GeneratedText),
		EditedText:    strPtrOrNil(a.EditedText),
		LLMOutput:     llm,
		Model:         strPtrOrNil(a.Model),
		Error:         strPtrOrNil(a.Error),
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}
}

type applicationWithVacancyDTO struct {
	Application applicationDTO `json:"application"`
	Vacancy     vacancyDTO     `json:"vacancy"`
}

func newApplicationWithVacancyDTO(av storage.ApplicationWithVacancy) applicationWithVacancyDTO {
	return applicationWithVacancyDTO{
		Application: newApplicationDTO(av.Application),
		Vacancy:     newVacancyDTO(av.Vacancy),
	}
}

// listItemDTO mirrors frontend/src/types.ts ListItem.
type listItemDTO struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Company   string    `json:"company"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Preview   string    `json:"preview"`
}

func newListItemDTO(it storage.ApplicationListItem) listItemDTO {
	return listItemDTO{
		ID:        it.ID,
		Title:     it.Title,
		Company:   it.Company,
		Status:    it.Status,
		CreatedAt: it.CreatedAt,
		Preview:   it.Preview,
	}
}

type listApplicationsResponse struct {
	Items []listItemDTO `json:"items"`
	Total int           `json:"total"`
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
