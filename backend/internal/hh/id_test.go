package hh

import "testing"

func TestParseVacancyID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"bare path", "hh.ru/vacancy/123", "123", false},
		{"https spb subdomain with query", "https://spb.hh.ru/vacancy/123?from=vacancy_search_list", "123", false},
		{"www subdomain", "https://www.hh.ru/vacancy/987654", "987654", false},
		{"hh.kz domain", "https://hh.kz/vacancy/555", "555", false},
		{"api url", "https://api.hh.ru/vacancies/42", "42", false},
		{"api url no scheme", "api.hh.ru/vacancies/42", "42", false},
		{"bare numeric id", "123456789", "123456789", false},
		{"bare numeric id with whitespace", "  42  ", "42", false},
		{"http scheme", "http://hh.ru/vacancy/1", "1", false},
		{"trailing slash and fragment", "https://hh.ru/vacancy/321#description", "321", false},
		{"empty string", "", "", true},
		{"not a vacancy url", "https://hh.ru/employer/12345", "", true},
		{"garbage", "not a url at all", "", true},
		{"vacancy without id", "https://hh.ru/vacancy/", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVacancyID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseVacancyID(%q) = %q, nil; want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVacancyID(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseVacancyID(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
