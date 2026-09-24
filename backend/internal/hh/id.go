// Package hh provides a client for the public hh.ru vacancy API and helpers
// for extracting vacancy identifiers from hh.ru URLs.
package hh

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidURL is returned by ParseVacancyID when the input does not
// contain a recognizable hh.ru vacancy identifier.
var ErrInvalidURL = errors.New("hh: invalid vacancy url")

// numericRe matches a bare (or embedded) sequence of digits used as a
// vacancy id.
var numericRe = regexp.MustCompile(`^\d+$`)

// vacancyPathRe matches "/vacancy/<id>" or "/vacancies/<id>" path segments,
// as used by hh.ru (web) and api.hh.ru (API) respectively.
var vacancyPathRe = regexp.MustCompile(`/vacanc(?:y|ies)/(\d+)`)

// ParseVacancyID extracts the numeric vacancy id from a hh.ru URL.
//
// It accepts, among others:
//   - hh.ru/vacancy/123
//   - https://spb.hh.ru/vacancy/123?from=...
//   - https://www.hh.ru/vacancy/123
//   - https://hh.kz/vacancy/123
//   - https://api.hh.ru/vacancies/123
//   - a bare numeric id, e.g. "123"
//
// It returns ErrInvalidURL if no vacancy id can be found.
func ParseVacancyID(rawURL string) (string, error) {
	s := strings.TrimSpace(rawURL)
	if s == "" {
		return "", ErrInvalidURL
	}

	if numericRe.MatchString(s) {
		return s, nil
	}

	if m := vacancyPathRe.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}

	return "", ErrInvalidURL
}
