package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo provides access to the vacancies/applications tables over a pgx pool.
type Repo struct {
	pool *pgxpool.Pool
}

// NewRepo builds a Repo around an already-connected pool.
func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// NewPool connects a pgx pool to dsn, pinging it before returning.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("storage: ping db: %w", err)
	}
	return pool, nil
}

// UpsertVacancyParams carries the fields needed to insert or refresh a vacancy.
type UpsertVacancyParams struct {
	HHID        string
	URL         string
	Title       string
	Company     string
	Salary      string
	Description string
	KeySkills   []string
	Raw         []byte
}

// UpsertVacancy inserts a vacancy or, if one with the same hh_id already
// exists, updates its mutable fields, returning the resulting row.
func (r *Repo) UpsertVacancy(ctx context.Context, p UpsertVacancyParams) (Vacancy, error) {
	const q = `
INSERT INTO vacancies (hh_id, url, title, company, salary, description, key_skills, raw, source)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'hh')
ON CONFLICT (hh_id) DO UPDATE SET
	url = EXCLUDED.url,
	title = EXCLUDED.title,
	company = EXCLUDED.company,
	salary = EXCLUDED.salary,
	description = EXCLUDED.description,
	key_skills = EXCLUDED.key_skills,
	raw = EXCLUDED.raw
RETURNING id, coalesce(hh_id, ''), coalesce(url, ''), title, company, salary, description, key_skills, raw, created_at, source`

	var v Vacancy
	err := r.pool.QueryRow(ctx, q, p.HHID, p.URL, p.Title, p.Company, p.Salary, p.Description, p.KeySkills, string(p.Raw)).
		Scan(&v.ID, &v.HHID, &v.URL, &v.Title, &v.Company, &v.Salary, &v.Description, &v.KeySkills, &v.Raw, &v.CreatedAt, &v.Source)
	if err != nil {
		return Vacancy{}, fmt.Errorf("storage: upsert vacancy: %w", err)
	}
	return v, nil
}

// InsertManualVacancyParams carries the fields needed to insert a manually
// entered vacancy (source = "manual"). Manual vacancies are never deduped:
// every submission creates a new row, unlike UpsertVacancy's hh_id upsert.
type InsertManualVacancyParams struct {
	Title       string
	Company     string
	Description string
}

// InsertManualVacancy inserts a new vacancy with source = "manual" and no
// hh_id/url, returning the resulting row.
func (r *Repo) InsertManualVacancy(ctx context.Context, p InsertManualVacancyParams) (Vacancy, error) {
	const q = `
INSERT INTO vacancies (hh_id, url, title, company, description, raw, source)
VALUES (NULL, NULL, $1, $2, $3, '{}', 'manual')
RETURNING id, coalesce(hh_id, ''), coalesce(url, ''), title, company, coalesce(salary, ''), description, key_skills, raw, created_at, source`

	var v Vacancy
	err := r.pool.QueryRow(ctx, q, p.Title, p.Company, p.Description).
		Scan(&v.ID, &v.HHID, &v.URL, &v.Title, &v.Company, &v.Salary, &v.Description, &v.KeySkills, &v.Raw, &v.CreatedAt, &v.Source)
	if err != nil {
		return Vacancy{}, fmt.Errorf("storage: insert manual vacancy: %w", err)
	}
	return v, nil
}

// CreateApplicationParams carries the fields needed to create a new application.
type CreateApplicationParams struct {
	VacancyID     int64
	FilterPassed  bool
	FilterReason  string
	Status        string
	GeneratedText string
	LLMOutput     []byte
	Model         string
	Error         string
}

// CreateApplication inserts a new application row and returns it.
func (r *Repo) CreateApplication(ctx context.Context, p CreateApplicationParams) (Application, error) {
	const q = `
INSERT INTO applications (vacancy_id, filter_passed, filter_reason, status, generated_text, llm_output, model, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, vacancy_id, filter_passed, coalesce(filter_reason, ''), status, coalesce(generated_text, ''),
	coalesce(edited_text, ''), llm_output, coalesce(model, ''), coalesce(error, ''), created_at, updated_at`

	var a Application
	err := r.pool.QueryRow(ctx, q, p.VacancyID, p.FilterPassed, nullIfEmpty(p.FilterReason), p.Status,
		nullIfEmpty(p.GeneratedText), nullIfEmpty(string(p.LLMOutput)), nullIfEmpty(p.Model), nullIfEmpty(p.Error)).
		Scan(&a.ID, &a.VacancyID, &a.FilterPassed, &a.FilterReason, &a.Status, &a.GeneratedText,
			&a.EditedText, &a.LLMOutput, &a.Model, &a.Error, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Application{}, fmt.Errorf("storage: create application: %w", err)
	}
	return a, nil
}

// ListApplications returns a page of applications, newest first, joined with
// their vacancy's title/company and a short letter preview.
func (r *Repo) ListApplications(ctx context.Context, limit, offset int) ([]ApplicationListItem, error) {
	const q = `
SELECT a.id, coalesce(v.title, ''), coalesce(v.company, ''), a.status,
	left(coalesce(a.edited_text, a.generated_text, ''), 150), a.created_at
FROM applications a
JOIN vacancies v ON v.id = a.vacancy_id
ORDER BY a.created_at DESC
LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("storage: list applications: %w", err)
	}
	defer rows.Close()

	items := make([]ApplicationListItem, 0, limit)
	for rows.Next() {
		var it ApplicationListItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Company, &it.Status, &it.Preview, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("storage: scan application list item: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: list applications rows: %w", err)
	}
	return items, nil
}

// CountApplications returns the total number of applications, for pagination.
func (r *Repo) CountApplications(ctx context.Context) (int, error) {
	const q = `SELECT count(*) FROM applications`
	var n int
	if err := r.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("storage: count applications: %w", err)
	}
	return n, nil
}

// GetApplication fetches a single application together with its vacancy.
// Returns ErrNotFound if no application with the given id exists.
func (r *Repo) GetApplication(ctx context.Context, id int64) (ApplicationWithVacancy, error) {
	const q = `
SELECT
	a.id, a.vacancy_id, a.filter_passed, coalesce(a.filter_reason, ''), a.status, coalesce(a.generated_text, ''),
	coalesce(a.edited_text, ''), a.llm_output, coalesce(a.model, ''), coalesce(a.error, ''), a.created_at, a.updated_at,
	v.id, coalesce(v.hh_id, ''), coalesce(v.url, ''), v.title, v.company, coalesce(v.salary, ''), v.description, v.key_skills, v.raw, v.created_at, v.source
FROM applications a
JOIN vacancies v ON v.id = a.vacancy_id
WHERE a.id = $1`

	var res ApplicationWithVacancy
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&res.Application.ID, &res.Application.VacancyID, &res.Application.FilterPassed, &res.Application.FilterReason,
		&res.Application.Status, &res.Application.GeneratedText, &res.Application.EditedText, &res.Application.LLMOutput,
		&res.Application.Model, &res.Application.Error, &res.Application.CreatedAt, &res.Application.UpdatedAt,
		&res.Vacancy.ID, &res.Vacancy.HHID, &res.Vacancy.URL, &res.Vacancy.Title, &res.Vacancy.Company,
		&res.Vacancy.Salary, &res.Vacancy.Description, &res.Vacancy.KeySkills, &res.Vacancy.Raw, &res.Vacancy.CreatedAt, &res.Vacancy.Source,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ApplicationWithVacancy{}, ErrNotFound
		}
		return ApplicationWithVacancy{}, fmt.Errorf("storage: get application: %w", err)
	}
	return res, nil
}

// UpdateEditedText saves the user's edited letter text for an application
// and bumps updated_at. Returns ErrNotFound if no such application exists.
func (r *Repo) UpdateEditedText(ctx context.Context, id int64, editedText string) error {
	const q = `UPDATE applications SET edited_text = $2, updated_at = now() WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, editedText)
	if err != nil {
		return fmt.Errorf("storage: update edited text: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// nullIfEmpty converts an empty string to nil so it is stored as SQL NULL
// rather than an empty string.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
