// Command try is a manual script for checking cover-letter generation
// quality. It reads a vacancy from a text file (title on the first line,
// description on the rest), generates a letter with OpenAIGenerator using
// the project's resume and template, and prints the JSON output plus the
// letter text.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/egorgorban/soprovod/backend/internal/letter"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: try <path-to-vacancy-text-file>")
	}
	vacancyPath := os.Args[1]

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is not set")
	}
	model := os.Getenv("OPENAI_MODEL")

	resumePath := envOr("RESUME_PATH", "../resume/resume.md")
	templatePath := envOr("TEMPLATE_PATH", "../resume/template.md")

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		return fmt.Errorf("read resume: %w", err)
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	vacancyRaw, err := os.ReadFile(vacancyPath)
	if err != nil {
		return fmt.Errorf("read vacancy file: %w", err)
	}

	title, description := splitVacancy(string(vacancyRaw))

	gen := letter.NewOpenAIGenerator(apiKey, model, string(resume), string(template))

	res, err := gen.Generate(context.Background(), letter.Vacancy{
		Title:       title,
		Description: description,
	})
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	out, err := json.MarshalIndent(res.Output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(out))
	fmt.Println()
	fmt.Println("--- letter ---")
	fmt.Println(res.Output.Letter)

	return nil
}

// splitVacancy splits a vacancy text file into a title (first line) and the
// rest as description.
func splitVacancy(raw string) (title, description string) {
	lines := strings.SplitN(strings.TrimLeft(raw, "\n"), "\n", 2)
	title = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		description = strings.TrimSpace(lines[1])
	}
	return title, description
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
