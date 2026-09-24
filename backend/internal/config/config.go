// Package config loads runtime configuration from environment variables.
package config

import "os"

// Config holds all environment-derived settings for the service.
type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	OpenAIAPIKey string
	OpenAIModel  string
	ResumePath   string
	TemplatePath string
	StaticDir    string
	HHSource     string
}

// Load reads configuration from the environment, applying defaults where
// values are not set. It does not validate that required values (e.g.
// DatabaseURL) are present -- callers should check that themselves.
func Load() Config {
	return Config{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:  getEnv("OPENAI_MODEL", "gpt-5-mini"),
		ResumePath:   getEnv("RESUME_PATH", "resume/resume.md"),
		TemplatePath: getEnv("TEMPLATE_PATH", "resume/template.md"),
		StaticDir:    getEnv("STATIC_DIR", "frontend/dist"),
		HHSource:     getEnv("HH_SOURCE", "html"),
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
