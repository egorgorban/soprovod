package letter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// defaultModel is used when no model is explicitly configured. gpt-5-mini is
// a reasoning model; such models reject the
// `temperature` parameter, so OpenAIGenerator never sets it.
const defaultModel = "gpt-5-mini"

// reasoningModelPrefixes lists model name prefixes that are reasoning
// models and support (or require omitting) reasoning_effort / temperature.
var reasoningModelPrefixes = []string{"gpt-5", "o1", "o3", "o4"}

// OpenAIGenerator generates cover letters via the OpenAI Chat Completions
// API using structured (JSON Schema) output.
type OpenAIGenerator struct {
	client   openai.Client
	apiKey   string
	model    string
	resume   string
	template string
}

// Option configures an OpenAIGenerator.
type Option func(*OpenAIGenerator)

// WithBaseURL overrides the OpenAI API base URL, primarily for tests against
// a fake server.
func WithBaseURL(baseURL string) Option {
	return func(g *OpenAIGenerator) {
		g.client = openai.NewClient(
			option.WithAPIKey(apiKeyOrPlaceholder(g)),
			option.WithBaseURL(baseURL),
		)
	}
}

// apiKeyOrPlaceholder is a small helper so WithBaseURL can be applied after
// NewOpenAIGenerator already built a client with the real key; we keep the
// original key stashed on the generator via a closure captured at
// construction time instead. See NewOpenAIGenerator.
func apiKeyOrPlaceholder(g *OpenAIGenerator) string {
	return g.apiKey
}

// NewOpenAIGenerator creates a Generator backed by the OpenAI API.
//
// apiKey is the OpenAI API key. model is the model name (e.g. "gpt-5-nano");
// if empty, defaultModel is used. resume and template are the raw contents
// of resume/resume.md and resume/template.md.
func NewOpenAIGenerator(apiKey, model, resume, template string, opts ...Option) *OpenAIGenerator {
	if model == "" {
		model = defaultModel
	}
	g := &OpenAIGenerator{
		model:    model,
		resume:   resume,
		template: template,
		apiKey:   apiKey,
	}
	g.client = openai.NewClient(option.WithAPIKey(apiKey))
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate calls the OpenAI API to produce a cover letter for the vacancy.
func (g *OpenAIGenerator) Generate(ctx context.Context, v Vacancy) (*Result, error) {
	params := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(g.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(buildSystemPrompt(g.resume, g.template)),
			openai.UserMessage(buildUserPrompt(v)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "cover_letter",
					Strict: openai.Bool(true),
					Schema: jsonSchema,
				},
			},
		},
	}

	// Reasoning models (gpt-5*, o1/o3/o4) reject `temperature` and support
	// `reasoning_effort` instead; we never set temperature and set a low
	// reasoning effort for these models to keep latency/cost down.
	if isReasoningModel(g.model) {
		params.ReasoningEffort = shared.ReasoningEffortLow
	}

	// The model occasionally breaks formatting rules (e.g. dumps JSON fields
	// into the letter), so one retry is allowed on validation failure.
	var lastErr error
	for range maxAttempts {
		resp, err := g.client.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("letter: openai chat completion: %w", err)
		}
		if len(resp.Choices) == 0 {
			return nil, errors.New("letter: openai returned no choices")
		}

		content := resp.Choices[0].Message.Content
		var out Output
		if err := json.Unmarshal([]byte(content), &out); err != nil {
			return nil, fmt.Errorf("letter: unmarshal structured output: %w", err)
		}
		if err := validateOutput(out); err != nil {
			lastErr = err
			continue
		}

		return &Result{
			Output: out,
			Raw:    json.RawMessage(content),
			Model:  resp.Model,
		}, nil
	}
	return nil, lastErr
}

const maxAttempts = 2

// leakedMarkers are fragments that must never appear in a letter: JSON field
// names and list formatting the model sometimes copies from the output schema.
var leakedMarkers = []string{"relevant_experience", "source:", "text:", "\n- ", "\n* "}

func validateOutput(out Output) error {
	if strings.TrimSpace(out.Letter) == "" {
		return errors.New("letter: generated letter is empty")
	}
	for _, m := range leakedMarkers {
		if strings.Contains(out.Letter, m) {
			return fmt.Errorf("letter: generated letter contains %q", m)
		}
	}
	return nil
}

func isReasoningModel(model string) bool {
	for _, p := range reasoningModelPrefixes {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}
