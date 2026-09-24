package letter

// jsonSchema is the strict JSON Schema describing Output, used as the
// structured-output schema for the OpenAI request.
var jsonSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"role": map[string]any{
			"type":        "string",
			"description": "Роль кандидата применительно к вакансии, например 'Go-разработчик'",
		},
		"years": map[string]any{
			"type":        "integer",
			"description": "Суммарный опыт кандидата в годах, из резюме",
		},
		"stack": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Пересечение стека вакансии и резюме",
		},
		"relevant_experience": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source": map[string]any{"type": "string"},
					"text":   map[string]any{"type": "string"},
				},
				"required":             []string{"source", "text"},
				"additionalProperties": false,
			},
			"description": "1-3 релевантных пункта опыта из резюме",
		},
		"letter": map[string]any{
			"type":        "string",
			"description": "Полный текст сопроводительного письма, 500-900 символов",
		},
	},
	"required":             []string{"role", "years", "stack", "relevant_experience", "letter"},
	"additionalProperties": false,
}
