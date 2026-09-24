// Package filter decides whether a vacancy is worth applying to.
package filter

import "context"

// Input describes a vacancy to be checked by a Filter.
type Input struct {
	Title       string
	Company     string
	Description string
	KeySkills   []string
}

// Result is the outcome of a filter check.
type Result struct {
	Pass   bool
	Reason string
}

// Filter decides whether a vacancy passes and should proceed to letter generation.
type Filter interface {
	Check(ctx context.Context, in Input) (Result, error)
}

// MockFilter is a stub Filter that always passes every vacancy.
type MockFilter struct{}

// Check always returns {Pass: true, Reason: "mock"}.
func (MockFilter) Check(_ context.Context, _ Input) (Result, error) {
	return Result{Pass: true, Reason: "mock"}, nil
}
