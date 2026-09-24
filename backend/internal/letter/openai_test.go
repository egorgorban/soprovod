package letter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testResume = "# Резюме\nПитон и Go разработчик, 5 лет опыта. Стек: Python, Go, PostgreSQL, Kafka."
const testTemplate = "Добрый день!\n\nЯ %роль% с опытом %количество%+ лет. Основной стек: %стек%.\n\n%опыт, подходящий вакансии%\n\nБуду рад обсудить дальнейшие шаги!"

// canned structured-output response body content (the JSON that would be
// inside choices[0].message.content).
const cannedContent = `{"role":"Go-разработчик","years":5,"stack":["Go","PostgreSQL"],"relevant_experience":[{"source":"Sberlabs","text":"Разработал API-Gateway на Go для облачных БД."}],"letter":"Добрый день!\n\nЯ Go-разработчик с опытом 5+ лет. Основной стек: Go, PostgreSQL.\n\nСейчас ищу предложения связанные с Go, PostgreSQL, поэтому заинтересовала ваша вакансия.\n\nВ Sberlabs разработал API-Gateway на Go для облачных БД.\n\nБуду рад обсудить дальнейшие шаги!"}`

func newFakeServer(t *testing.T, gotBody *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*gotBody = body

		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-5-nano",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": cannedContent,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestOpenAIGenerator_Generate(t *testing.T) {
	var gotBody []byte
	srv := newFakeServer(t, &gotBody)
	defer srv.Close()

	gen := NewOpenAIGenerator("test-key", "gpt-5-nano", testResume, testTemplate, WithBaseURL(srv.URL+"/"))

	v := Vacancy{
		Title:       "Backend Developer (Go)",
		Company:     "Acme",
		Description: "Ищем Go-разработчика с опытом PostgreSQL и Kafka.",
		KeySkills:   []string{"Go", "PostgreSQL", "Kafka"},
	}

	res, err := gen.Generate(context.Background(), v)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if res.Output.Letter == "" {
		t.Fatal("expected non-empty letter")
	}
	if res.Output.Role != "Go-разработчик" {
		t.Errorf("unexpected role: %q", res.Output.Role)
	}
	if res.Model != "gpt-5-nano" {
		t.Errorf("unexpected model: %q", res.Model)
	}
	if len(res.Output.RelevantExperience) == 0 {
		t.Error("expected relevant experience")
	}

	// The prompt sent to the API must contain both the resume and the vacancy info.
	bodyStr := string(gotBody)
	if !strings.Contains(bodyStr, "Питон и Go разработчик") {
		t.Error("request body does not contain resume content")
	}
	if !strings.Contains(bodyStr, "Backend Developer (Go)") {
		t.Error("request body does not contain vacancy title")
	}
	if !strings.Contains(bodyStr, "Ищем Go-разработчика") {
		t.Error("request body does not contain vacancy description")
	}
}

func TestOpenAIGenerator_EmptyLetterFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-5-nano",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"role":"x","years":0,"stack":[],"relevant_experience":[],"letter":""}`,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	gen := NewOpenAIGenerator("test-key", "gpt-5-nano", testResume, testTemplate, WithBaseURL(srv.URL+"/"))
	_, err := gen.Generate(context.Background(), Vacancy{Title: "T"})
	if err == nil {
		t.Fatal("expected error for empty letter")
	}
}
