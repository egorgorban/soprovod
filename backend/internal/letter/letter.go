// Package letter generates a cover letter for a vacancy using an LLM,
// following the resume and the cover-letter template.
package letter

import (
	"context"
	"encoding/json"
)

// Vacancy is the input vacancy data used to generate a cover letter.
type Vacancy struct {
	Title       string
	Company     string
	Description string
	KeySkills   []string
}

// Experience is one piece of relevant experience pulled from the resume.
type Experience struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

// Output is the structured result produced by the LLM.
type Output struct {
	Role               string       `json:"role"`
	Years              int          `json:"years"`
	Stack              []string     `json:"stack"`
	RelevantExperience []Experience `json:"relevant_experience"`
	Letter             string       `json:"letter"`
}

// Result wraps the parsed Output together with the raw model response and
// the model name that produced it, for persistence/debugging.
type Result struct {
	Output Output
	Raw    json.RawMessage
	Model  string
}

// Generator produces a cover letter Result for a given Vacancy.
type Generator interface {
	Generate(ctx context.Context, vacancy Vacancy) (*Result, error)
}
