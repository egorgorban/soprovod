package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/egorgorban/soprovod/backend/internal/hhparser"
	"github.com/egorgorban/soprovod/backend/internal/pipeline"
	"github.com/egorgorban/soprovod/backend/internal/storage"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
	// maxManualTextRunes bounds the size of a manually pasted vacancy text.
	maxManualTextRunes = 50_000
)

// Pipeline is the subset of pipeline.Service the HTTP layer needs.
type Pipeline interface {
	Process(ctx context.Context, rawURL string) (storage.ApplicationWithVacancy, error)
	ProcessText(ctx context.Context, mv pipeline.ManualVacancy) (storage.ApplicationWithVacancy, error)
}

// ApplicationRepo is the subset of storage.Repo the HTTP layer needs, beyond
// what the pipeline uses to create applications.
type ApplicationRepo interface {
	ListApplications(ctx context.Context, limit, offset int) ([]storage.ApplicationListItem, error)
	CountApplications(ctx context.Context) (int, error)
	GetApplication(ctx context.Context, id int64) (storage.ApplicationWithVacancy, error)
	UpdateEditedText(ctx context.Context, id int64, editedText string) error
}

type createApplicationRequest struct {
	URL     string `json:"url"`
	Text    string `json:"text"`
	Title   string `json:"title"`
	Company string `json:"company"`
}

// handleCreateApplication accepts either {url} (fetch from hh.ru) or
// {text, title?, company?} (manually pasted vacancy). Exactly one of url/text
// must be set.
func handleCreateApplication(pl Pipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createApplicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "некорректное тело запроса")
			return
		}

		hasURL := strings.TrimSpace(req.URL) != ""
		hasText := strings.TrimSpace(req.Text) != ""

		switch {
		case hasURL && hasText:
			writeError(w, http.StatusBadRequest, "укажите либо ссылку, либо текст вакансии, не оба сразу")
			return
		case !hasURL && !hasText:
			writeError(w, http.StatusBadRequest, "укажите ссылку на вакансию или её текст")
			return
		case hasText:
			if utf8.RuneCountInString(req.Text) > maxManualTextRunes {
				writeError(w, http.StatusBadRequest, "текст вакансии слишком длинный (максимум 50000 символов)")
				return
			}
			res, err := pl.ProcessText(r.Context(), pipeline.ManualVacancy{
				Text:    req.Text,
				Title:   req.Title,
				Company: req.Company,
			})
			if err != nil {
				writeProcessError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, newApplicationWithVacancyDTO(res))
		default:
			res, err := pl.Process(r.Context(), req.URL)
			if err != nil {
				writeProcessError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, newApplicationWithVacancyDTO(res))
		}
	}
}

func writeProcessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pipeline.ErrInvalidURL):
		writeError(w, http.StatusBadRequest, "invalid vacancy url")
	case errors.Is(err, pipeline.ErrVacancyNotFound):
		writeError(w, http.StatusNotFound, "vacancy not found")
	case errors.Is(err, hhparser.ErrBlocked):
		writeError(w, http.StatusBadGateway, "hh.ru заблокировал запрос (капча), попробуйте позже")
	case errors.Is(err, pipeline.ErrUpstream):
		writeError(w, http.StatusBadGateway, "upstream service unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func handleListApplications(repo ApplicationRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultListLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			limit = n
		}
		if limit > maxListLimit {
			limit = maxListLimit
		}

		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "invalid offset")
				return
			}
			offset = n
		}

		items, err := repo.ListApplications(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		total, err := repo.CountApplications(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		dtoItems := make([]listItemDTO, 0, len(items))
		for _, it := range items {
			dtoItems = append(dtoItems, newListItemDTO(it))
		}

		writeJSON(w, http.StatusOK, listApplicationsResponse{Items: dtoItems, Total: total})
	}
}

func handleGetApplication(repo ApplicationRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		res, err := repo.GetApplication(r.Context(), id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusNotFound, "application not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, newApplicationWithVacancyDTO(res))
	}
}

type updateApplicationRequest struct {
	EditedText string `json:"edited_text"`
}

type updateApplicationResponse struct {
	Application applicationDTO `json:"application"`
}

func handleUpdateApplication(repo ApplicationRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		var req updateApplicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := repo.UpdateEditedText(r.Context(), id, req.EditedText); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusNotFound, "application not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		res, err := repo.GetApplication(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, updateApplicationResponse{Application: newApplicationDTO(res.Application)})
	}
}

func parseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
