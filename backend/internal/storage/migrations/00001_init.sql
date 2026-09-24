-- +goose Up
CREATE TABLE vacancies (
    id bigserial PRIMARY KEY,
    hh_id text UNIQUE NOT NULL,
    url text NOT NULL,
    title text,
    company text,
    salary text,
    description text NOT NULL,
    key_skills text[],
    raw jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE applications (
    id bigserial PRIMARY KEY,
    vacancy_id bigint NOT NULL REFERENCES vacancies (id),
    filter_passed boolean NOT NULL,
    filter_reason text,
    status text NOT NULL,
    generated_text text,
    edited_text text,
    llm_output jsonb,
    model text,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_applications_created_at ON applications (created_at DESC);

-- +goose Down
DROP TABLE applications;
DROP TABLE vacancies;
