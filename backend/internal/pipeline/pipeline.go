// Package pipeline orchestrates the fetch -> filter -> generate -> save flow
// for a single vacancy URL. It knows nothing about HTTP: the same Service is
// meant to be reusable from a future cron worker.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/egorgorban/soprovod/backend/internal/filter"
	"github.com/egorgorban/soprovod/backend/internal/hh"
	"github.com/egorgorban/soprovod/backend/internal/letter"
	"github.com/egorgorban/soprovod/backend/internal/storage"
)

// maxManualTitleRunes bounds the fallback title derived from the first
// non-empty line of a manually pasted vacancy text.
const maxManualTitleRunes = 120

// ErrInvalidURL is returned when the given string does not contain a
// recognizable hh.ru vacancy id. Maps to HTTP 400.
var ErrInvalidURL = errors.New("pipeline: invalid vacancy url")

// ErrVacancyNotFound is returned when hh.ru reports the vacancy does not
// exist. Maps to HTTP 404.
var ErrVacancyNotFound = errors.New("pipeline: vacancy not found")

// ErrUpstream wraps failures talking to hh.ru or the LLM provider that are
// not the caller's fault. Maps to HTTP 502.
var ErrUpstream = errors.New("pipeline: upstream error")

// VacancyFetcher fetches a vacancy from hh.ru. Satisfied by *hh.Client.
type VacancyFetcher interface {
	GetVacancy(ctx context.Context, id string) (*hh.Vacancy, error)
}

// Repo is the subset of storage.Repo the pipeline needs.
type Repo interface {
	UpsertVacancy(ctx context.Context, p storage.UpsertVacancyParams) (storage.Vacancy, error)
	InsertManualVacancy(ctx context.Context, p storage.InsertManualVacancyParams) (storage.Vacancy, error)
	CreateApplication(ctx context.Context, p storage.CreateApplicationParams) (storage.Application, error)
}

// ManualVacancy is the input for ProcessText: a vacancy pasted by hand
// instead of fetched from hh.ru.
type ManualVacancy struct {
	Text    string
	Title   string
	Company string
}

// Service runs the vacancy -> letter pipeline.
type Service struct {
	Fetcher   VacancyFetcher
	Filter    filter.Filter
	Generator letter.Generator
	Repo      Repo
	Logger    *slog.Logger
}

// NewService builds a Service. logger may be nil, in which case slog.Default() is used.
func NewService(fetcher VacancyFetcher, f filter.Filter, gen letter.Generator, repo Repo, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{Fetcher: fetcher, Filter: f, Generator: gen, Repo: repo, Logger: logger}
}

// Process runs the full pipeline for a hh.ru vacancy URL (or bare id):
// parse -> fetch -> upsert vacancy -> filter -> (if passed) generate letter
// -> persist an application row reflecting the outcome.
//
// Filter failures and generator failures are persisted as filtered_out /
// failed applications respectively and returned successfully (not as an
// error) so the caller can show the resulting application. Only
// infrastructure failures (invalid url, vacancy not found, hh/LLM transport
// errors, DB errors) are returned as errors.
func (s *Service) Process(ctx context.Context, rawURL string) (storage.ApplicationWithVacancy, error) {
	id, err := hh.ParseVacancyID(rawURL)
	if err != nil {
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	hhVacancy, err := s.Fetcher.GetVacancy(ctx, id)
	if err != nil {
		if errors.Is(err, hh.ErrNotFound) {
			return storage.ApplicationWithVacancy{}, ErrVacancyNotFound
		}
		// Wrap the underlying error with %w (not %v) here, unlike the other
		// ErrUpstream wraps below: callers need errors.Is to see through to
		// fetcher-specific sentinels like hhparser.ErrBlocked/ErrParse so
		// the HTTP layer can map them to a more specific response.
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: fetch vacancy: %w", ErrUpstream, err)
	}

	vacancy, err := s.Repo.UpsertVacancy(ctx, storage.UpsertVacancyParams{
		HHID:        hhVacancy.ID,
		URL:         firstNonEmpty(hhVacancy.URL, rawURL),
		Title:       hhVacancy.Title,
		Company:     hhVacancy.Company,
		Salary:      hhVacancy.Salary,
		Description: hhVacancy.Description,
		KeySkills:   hhVacancy.KeySkills,
		Raw:         hhVacancy.Raw,
	})
	if err != nil {
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: save vacancy: %v", ErrUpstream, err)
	}

	return s.filterGenerateSave(ctx, vacancy)
}

// ProcessText runs the filter -> generate -> save tail of the pipeline for a
// manually pasted vacancy, skipping the hh.ru fetch step. Title falls back
// to the first non-empty line of the text (capped at maxManualTitleRunes
// runes) when mv.Title is empty; the description is the trimmed text as-is.
func (s *Service) ProcessText(ctx context.Context, mv ManualVacancy) (storage.ApplicationWithVacancy, error) {
	description := strings.TrimSpace(mv.Text)
	title := strings.TrimSpace(mv.Title)
	if title == "" {
		title = fallbackTitle(description)
	}

	vacancy, err := s.Repo.InsertManualVacancy(ctx, storage.InsertManualVacancyParams{
		Title:       title,
		Company:     strings.TrimSpace(mv.Company),
		Description: description,
	})
	if err != nil {
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: save vacancy: %v", ErrUpstream, err)
	}

	return s.filterGenerateSave(ctx, vacancy)
}

// fallbackTitle returns the first non-empty line of text, truncated to
// maxManualTitleRunes runes.
func fallbackTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= maxManualTitleRunes {
			return line
		}
		runes := []rune(line)
		return string(runes[:maxManualTitleRunes])
	}
	return ""
}

// filterGenerateSave runs the shared tail of both Process and ProcessText:
// filter -> (if passed) generate letter -> persist the resulting
// application row.
func (s *Service) filterGenerateSave(ctx context.Context, vacancy storage.Vacancy) (storage.ApplicationWithVacancy, error) {
	filterResult, err := s.Filter.Check(ctx, filter.Input{
		Title:       vacancy.Title,
		Company:     vacancy.Company,
		Description: vacancy.Description,
		KeySkills:   vacancy.KeySkills,
	})
	if err != nil {
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: filter: %v", ErrUpstream, err)
	}

	if !filterResult.Pass {
		app, err := s.Repo.CreateApplication(ctx, storage.CreateApplicationParams{
			VacancyID:    vacancy.ID,
			FilterPassed: false,
			FilterReason: filterResult.Reason,
			Status:       storage.StatusFilteredOut,
		})
		if err != nil {
			return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: save application: %v", ErrUpstream, err)
		}
		return storage.ApplicationWithVacancy{Application: app, Vacancy: vacancy}, nil
	}

	genResult, genErr := s.Generator.Generate(ctx, letter.Vacancy{
		Title:       vacancy.Title,
		Company:     vacancy.Company,
		Description: vacancy.Description,
		KeySkills:   vacancy.KeySkills,
	})

	params := storage.CreateApplicationParams{
		VacancyID:    vacancy.ID,
		FilterPassed: true,
		FilterReason: filterResult.Reason,
	}
	if genErr != nil {
		s.Logger.Error("letter generation failed", "vacancy_id", vacancy.ID, "error", genErr)
		params.Status = storage.StatusFailed
		params.Error = genErr.Error()
	} else {
		params.Status = storage.StatusGenerated
		params.GeneratedText = genResult.Output.Letter
		params.Model = genResult.Model
		if len(genResult.Raw) > 0 {
			params.LLMOutput = []byte(genResult.Raw)
		} else if b, err := json.Marshal(genResult.Output); err == nil {
			params.LLMOutput = b
		}
	}

	app, err := s.Repo.CreateApplication(ctx, params)
	if err != nil {
		return storage.ApplicationWithVacancy{}, fmt.Errorf("%w: save application: %v", ErrUpstream, err)
	}
	return storage.ApplicationWithVacancy{Application: app, Vacancy: vacancy}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
