package storage_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/egorgorban/soprovod/backend/internal/storage"
)

// setupRepo connects to TEST_DATABASE_URL, runs migrations, and returns a
// Repo. Tests are skipped entirely when TEST_DATABASE_URL is not set, so
// `go test ./...` works without a running postgres.
func setupRepo(t *testing.T) *storage.Repo {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping storage integration tests")
	}

	if err := storage.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := storage.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return storage.NewRepo(pool)
}

func uniqueHHID(t *testing.T) string {
	t.Helper()
	return t.Name() + "-" + time.Now().Format("150405.000000000")
}

func TestUpsertVacancy(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	hhID := uniqueHHID(t)
	params := storage.UpsertVacancyParams{
		HHID:        hhID,
		URL:         "https://hh.ru/vacancy/1",
		Title:       "Backend Developer",
		Company:     "Acme",
		Salary:      "200000",
		Description: "do stuff",
		KeySkills:   []string{"Go", "PostgreSQL"},
		Raw:         []byte(`{"id":"1"}`),
	}

	v, err := repo.UpsertVacancy(ctx, params)
	if err != nil {
		t.Fatalf("upsert vacancy: %v", err)
	}
	if v.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if v.Title != "Backend Developer" {
		t.Errorf("title = %q, want %q", v.Title, "Backend Developer")
	}

	// Upserting again with same hh_id should update, not duplicate.
	params.Title = "Senior Backend Developer"
	v2, err := repo.UpsertVacancy(ctx, params)
	if err != nil {
		t.Fatalf("second upsert vacancy: %v", err)
	}
	if v2.ID != v.ID {
		t.Errorf("expected same id on upsert, got %d and %d", v.ID, v2.ID)
	}
	if v2.Title != "Senior Backend Developer" {
		t.Errorf("title = %q, want updated title", v2.Title)
	}
}

func TestApplicationLifecycle(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	v, err := repo.UpsertVacancy(ctx, storage.UpsertVacancyParams{
		HHID:        uniqueHHID(t),
		URL:         "https://hh.ru/vacancy/2",
		Title:       "Go Developer",
		Company:     "Beta",
		Description: "desc",
		Raw:         []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("upsert vacancy: %v", err)
	}

	longText := ""
	for i := 0; i < 200; i++ {
		longText += "a"
	}

	app, err := repo.CreateApplication(ctx, storage.CreateApplicationParams{
		VacancyID:     v.ID,
		FilterPassed:  true,
		FilterReason:  "mock",
		Status:        storage.StatusGenerated,
		GeneratedText: longText,
		LLMOutput:     []byte(`{"letter":"hi"}`),
		Model:         "gpt-5-nano",
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if app.ID == 0 {
		t.Fatal("expected non-zero application id")
	}

	got, err := repo.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if got.Vacancy.ID != v.ID {
		t.Errorf("vacancy id = %d, want %d", got.Vacancy.ID, v.ID)
	}
	if got.Application.Status != storage.StatusGenerated {
		t.Errorf("status = %q, want %q", got.Application.Status, storage.StatusGenerated)
	}

	if err := repo.UpdateEditedText(ctx, app.ID, "edited letter"); err != nil {
		t.Fatalf("update edited text: %v", err)
	}

	got2, err := repo.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application after edit: %v", err)
	}
	if got2.Application.EditedText != "edited letter" {
		t.Errorf("edited_text = %q, want %q", got2.Application.EditedText, "edited letter")
	}
	if !got2.Application.UpdatedAt.After(got.Application.UpdatedAt) {
		t.Errorf("expected updated_at to advance after edit")
	}

	items, err := repo.ListApplications(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list applications: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == app.ID {
			found = true
			if it.Title != "Go Developer" {
				t.Errorf("list title = %q, want %q", it.Title, "Go Developer")
			}
			if len(it.Preview) > 150 {
				t.Errorf("preview longer than 150 chars: %d", len(it.Preview))
			}
		}
	}
	if !found {
		t.Errorf("created application %d not found in list", app.ID)
	}
}

func TestInsertManualVacancy(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	v, err := repo.InsertManualVacancy(ctx, storage.InsertManualVacancyParams{
		Title:       "Go Developer (manual)",
		Company:     "Gamma",
		Description: "текст вакансии вставленный вручную",
	})
	if err != nil {
		t.Fatalf("insert manual vacancy: %v", err)
	}
	if v.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if v.Source != storage.VacancySourceManual {
		t.Errorf("source = %q, want %q", v.Source, storage.VacancySourceManual)
	}
	if v.HHID != "" {
		t.Errorf("hh_id = %q, want empty", v.HHID)
	}
	if v.URL != "" {
		t.Errorf("url = %q, want empty", v.URL)
	}

	// Inserting again should create a new row, not dedupe.
	v2, err := repo.InsertManualVacancy(ctx, storage.InsertManualVacancyParams{
		Title:       "Go Developer (manual)",
		Company:     "Gamma",
		Description: "текст вакансии вставленный вручную",
	})
	if err != nil {
		t.Fatalf("second insert manual vacancy: %v", err)
	}
	if v2.ID == v.ID {
		t.Error("expected a new row on second insert, not a dedupe")
	}

	app, err := repo.CreateApplication(ctx, storage.CreateApplicationParams{
		VacancyID:     v.ID,
		FilterPassed:  true,
		FilterReason:  "mock",
		Status:        storage.StatusGenerated,
		GeneratedText: "hello",
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	got, err := repo.GetApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if got.Vacancy.Source != storage.VacancySourceManual {
		t.Errorf("source = %q, want manual", got.Vacancy.Source)
	}
	if got.Vacancy.URL != "" || got.Vacancy.HHID != "" {
		t.Errorf("expected empty hh_id/url, got hh_id=%q url=%q", got.Vacancy.HHID, got.Vacancy.URL)
	}
}

func TestGetApplicationNotFound(t *testing.T) {
	repo := setupRepo(t)
	_, err := repo.GetApplication(context.Background(), 9_999_999)
	if err != storage.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateEditedTextNotFound(t *testing.T) {
	repo := setupRepo(t)
	err := repo.UpdateEditedText(context.Background(), 9_999_999, "x")
	if err != storage.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
