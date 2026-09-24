package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/egorgorban/soprovod/backend/internal/filter"
	"github.com/egorgorban/soprovod/backend/internal/hh"
	"github.com/egorgorban/soprovod/backend/internal/letter"
	"github.com/egorgorban/soprovod/backend/internal/pipeline"
	"github.com/egorgorban/soprovod/backend/internal/storage"
)

// fakeFetcher

type fakeFetcher struct {
	vacancy *hh.Vacancy
	err     error
}

func (f *fakeFetcher) GetVacancy(_ context.Context, _ string) (*hh.Vacancy, error) {
	return f.vacancy, f.err
}

// fakeFilter

type fakeFilter struct {
	result filter.Result
	err    error
}

func (f *fakeFilter) Check(_ context.Context, _ filter.Input) (filter.Result, error) {
	return f.result, f.err
}

// fakeGenerator

type fakeGenerator struct {
	result *letter.Result
	err    error
}

func (f *fakeGenerator) Generate(_ context.Context, _ letter.Vacancy) (*letter.Result, error) {
	return f.result, f.err
}

// fakeRepo

type fakeRepo struct {
	vacancy       storage.Vacancy
	upsertErr     error
	createErr     error
	createdParams []storage.CreateApplicationParams
	nextID        int64
}

func (r *fakeRepo) UpsertVacancy(_ context.Context, p storage.UpsertVacancyParams) (storage.Vacancy, error) {
	if r.upsertErr != nil {
		return storage.Vacancy{}, r.upsertErr
	}
	r.vacancy = storage.Vacancy{
		ID:          1,
		HHID:        p.HHID,
		URL:         p.URL,
		Title:       p.Title,
		Company:     p.Company,
		Salary:      p.Salary,
		Description: p.Description,
		KeySkills:   p.KeySkills,
		Raw:         p.Raw,
	}
	return r.vacancy, nil
}

func (r *fakeRepo) CreateApplication(_ context.Context, p storage.CreateApplicationParams) (storage.Application, error) {
	if r.createErr != nil {
		return storage.Application{}, r.createErr
	}
	r.nextID++
	r.createdParams = append(r.createdParams, p)
	return storage.Application{
		ID:            r.nextID,
		VacancyID:     p.VacancyID,
		FilterPassed:  p.FilterPassed,
		FilterReason:  p.FilterReason,
		Status:        p.Status,
		GeneratedText: p.GeneratedText,
		LLMOutput:     json.RawMessage(p.LLMOutput),
		Model:         p.Model,
		Error:         p.Error,
	}, nil
}

func validVacancy() *hh.Vacancy {
	return &hh.Vacancy{
		ID:          "123",
		URL:         "https://hh.ru/vacancy/123",
		Title:       "Go dev",
		Company:     "Acme",
		Description: "do stuff",
		KeySkills:   []string{"Go"},
		Raw:         json.RawMessage(`{"id":"123"}`),
	}
}

func TestProcess_Success(t *testing.T) {
	repo := &fakeRepo{}
	genResult := &letter.Result{
		Output: letter.Output{Letter: "hello"},
		Raw:    json.RawMessage(`{"letter":"hello"}`),
		Model:  "gpt-5-nano",
	}
	svc := pipeline.NewService(
		&fakeFetcher{vacancy: validVacancy()},
		&fakeFilter{result: filter.Result{Pass: true, Reason: "mock"}},
		&fakeGenerator{result: genResult},
		repo,
		nil,
	)

	res, err := svc.Process(context.Background(), "https://hh.ru/vacancy/123")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Application.Status != storage.StatusGenerated {
		t.Errorf("status = %q, want %q", res.Application.Status, storage.StatusGenerated)
	}
	if res.Application.GeneratedText != "hello" {
		t.Errorf("generated text = %q", res.Application.GeneratedText)
	}
	if res.Vacancy.Title != "Go dev" {
		t.Errorf("vacancy title = %q", res.Vacancy.Title)
	}
}

func TestProcess_FilteredOut(t *testing.T) {
	repo := &fakeRepo{}
	gen := &fakeGenerator{}
	svc := pipeline.NewService(
		&fakeFetcher{vacancy: validVacancy()},
		&fakeFilter{result: filter.Result{Pass: false, Reason: "not relevant"}},
		gen,
		repo,
		nil,
	)

	res, err := svc.Process(context.Background(), "https://hh.ru/vacancy/123")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Application.Status != storage.StatusFilteredOut {
		t.Errorf("status = %q, want %q", res.Application.Status, storage.StatusFilteredOut)
	}
	if len(repo.createdParams) != 1 || repo.createdParams[0].FilterPassed {
		t.Errorf("expected one filtered_out application, got %+v", repo.createdParams)
	}
}

func TestProcess_GeneratorFailure(t *testing.T) {
	repo := &fakeRepo{}
	svc := pipeline.NewService(
		&fakeFetcher{vacancy: validVacancy()},
		&fakeFilter{result: filter.Result{Pass: true, Reason: "mock"}},
		&fakeGenerator{err: errors.New("openai down")},
		repo,
		nil,
	)

	res, err := svc.Process(context.Background(), "https://hh.ru/vacancy/123")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.Application.Status != storage.StatusFailed {
		t.Errorf("status = %q, want %q", res.Application.Status, storage.StatusFailed)
	}
	if res.Application.Error == "" {
		t.Error("expected error text to be saved")
	}
}

func TestProcess_InvalidURL(t *testing.T) {
	svc := pipeline.NewService(&fakeFetcher{}, &fakeFilter{}, &fakeGenerator{}, &fakeRepo{}, nil)

	_, err := svc.Process(context.Background(), "not a url at all")
	if !errors.Is(err, pipeline.ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
}

func TestProcess_VacancyNotFound(t *testing.T) {
	svc := pipeline.NewService(&fakeFetcher{err: hh.ErrNotFound}, &fakeFilter{}, &fakeGenerator{}, &fakeRepo{}, nil)

	_, err := svc.Process(context.Background(), "https://hh.ru/vacancy/999")
	if !errors.Is(err, pipeline.ErrVacancyNotFound) {
		t.Fatalf("err = %v, want ErrVacancyNotFound", err)
	}
}

func TestProcess_FetchUpstreamError(t *testing.T) {
	svc := pipeline.NewService(&fakeFetcher{err: errors.New("timeout")}, &fakeFilter{}, &fakeGenerator{}, &fakeRepo{}, nil)

	_, err := svc.Process(context.Background(), "https://hh.ru/vacancy/999")
	if !errors.Is(err, pipeline.ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}
