package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egorgorban/soprovod/backend/internal/httpapi"
	"github.com/egorgorban/soprovod/backend/internal/pipeline"
	"github.com/egorgorban/soprovod/backend/internal/storage"
)

// fakePipeline

type fakePipeline struct {
	result storage.ApplicationWithVacancy
	err    error
}

func (f *fakePipeline) Process(_ context.Context, _ string) (storage.ApplicationWithVacancy, error) {
	return f.result, f.err
}

// fakeRepo

type fakeRepo struct {
	items      []storage.ApplicationListItem
	total      int
	getResult  storage.ApplicationWithVacancy
	getErr     error
	updateErr  error
	listErr    error
	countErr   error
	updateCall struct {
		id   int64
		text string
	}
}

func (f *fakeRepo) ListApplications(_ context.Context, _, _ int) ([]storage.ApplicationListItem, error) {
	return f.items, f.listErr
}

func (f *fakeRepo) CountApplications(_ context.Context) (int, error) {
	return f.total, f.countErr
}

func (f *fakeRepo) GetApplication(_ context.Context, _ int64) (storage.ApplicationWithVacancy, error) {
	return f.getResult, f.getErr
}

func (f *fakeRepo) UpdateEditedText(_ context.Context, id int64, text string) error {
	f.updateCall.id = id
	f.updateCall.text = text
	return f.updateErr
}

func sampleApplicationWithVacancy() storage.ApplicationWithVacancy {
	return storage.ApplicationWithVacancy{
		Application: storage.Application{
			ID:            1,
			VacancyID:     1,
			FilterPassed:  true,
			FilterReason:  "mock",
			Status:        storage.StatusGenerated,
			GeneratedText: "hello",
			LLMOutput:     json.RawMessage(`{"letter":"hello"}`),
			Model:         "gpt-5-nano",
		},
		Vacancy: storage.Vacancy{
			ID:          1,
			HHID:        "123",
			URL:         "https://hh.ru/vacancy/123",
			Title:       "Go dev",
			Company:     "Acme",
			Description: "desc",
			KeySkills:   []string{"Go"},
			Raw:         json.RawMessage(`{}`),
		},
	}
}

func newTestRouter(pl httpapi.Pipeline, repo httpapi.ApplicationRepo) http.Handler {
	return httpapi.NewRouter(httpapi.Deps{Pipeline: pl, Repo: repo})
}

func TestHandleCreateApplication_Success(t *testing.T) {
	pl := &fakePipeline{result: sampleApplicationWithVacancy()}
	r := newTestRouter(pl, &fakeRepo{})

	body, _ := json.Marshal(map[string]string{"url": "https://hh.ru/vacancy/123"})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Application struct {
			ID        int64       `json:"id"`
			LLMOutput interface{} `json:"llm_output"`
		} `json:"application"`
		Vacancy struct {
			KeySkills []string `json:"key_skills"`
		} `json:"vacancy"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Application.ID != 1 {
		t.Errorf("application.id = %d", resp.Application.ID)
	}
	if resp.Application.LLMOutput == nil {
		t.Error("expected llm_output to be a non-null object")
	}
	if resp.Vacancy.KeySkills == nil {
		t.Error("expected key_skills to never be null")
	}
}

func TestHandleCreateApplication_InvalidURL(t *testing.T) {
	pl := &fakePipeline{err: pipeline.ErrInvalidURL}
	r := newTestRouter(pl, &fakeRepo{})

	body, _ := json.Marshal(map[string]string{"url": "nope"})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleCreateApplication_MissingBody(t *testing.T) {
	r := newTestRouter(&fakePipeline{}, &fakeRepo{})

	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleCreateApplication_NotFound(t *testing.T) {
	pl := &fakePipeline{err: pipeline.ErrVacancyNotFound}
	r := newTestRouter(pl, &fakeRepo{})

	body, _ := json.Marshal(map[string]string{"url": "https://hh.ru/vacancy/999"})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleCreateApplication_Upstream(t *testing.T) {
	pl := &fakePipeline{err: pipeline.ErrUpstream}
	r := newTestRouter(pl, &fakeRepo{})

	body, _ := json.Marshal(map[string]string{"url": "https://hh.ru/vacancy/999"})
	req := httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestHandleListApplications(t *testing.T) {
	repo := &fakeRepo{
		items: []storage.ApplicationListItem{{ID: 1, Title: "Go dev", Company: "Acme", Status: "generated", Preview: "hi"}},
		total: 1,
	}
	r := newTestRouter(&fakePipeline{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/applications?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items []storage.ApplicationListItem `json:"items"`
		Total int                           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestHandleListApplications_LimitClamped(t *testing.T) {
	repo := &fakeRepo{}
	r := newTestRouter(&fakePipeline{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/applications?limit=1000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestHandleGetApplication_NotFound(t *testing.T) {
	repo := &fakeRepo{getErr: storage.ErrNotFound}
	r := newTestRouter(&fakePipeline{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHandleGetApplication_InvalidID(t *testing.T) {
	repo := &fakeRepo{}
	r := newTestRouter(&fakePipeline{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/applications/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandleUpdateApplication_Success(t *testing.T) {
	repo := &fakeRepo{getResult: sampleApplicationWithVacancy()}
	r := newTestRouter(&fakePipeline{}, repo)

	body, _ := json.Marshal(map[string]string{"edited_text": "new text"})
	req := httptest.NewRequest(http.MethodPatch, "/api/applications/1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if repo.updateCall.text != "new text" {
		t.Errorf("updateCall.text = %q", repo.updateCall.text)
	}
}

func TestHandleUpdateApplication_NotFound(t *testing.T) {
	repo := &fakeRepo{updateErr: storage.ErrNotFound}
	r := newTestRouter(&fakePipeline{}, repo)

	body, _ := json.Marshal(map[string]string{"edited_text": "x"})
	req := httptest.NewRequest(http.MethodPatch, "/api/applications/1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	r := newTestRouter(&fakePipeline{}, &fakeRepo{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}
