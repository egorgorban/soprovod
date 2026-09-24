# Soprovod — план реализации

Сервис для одного пользователя: ссылка на вакансию hh.ru → описание вакансии → сопроводительное письмо по шаблону с релевантным опытом из резюме → показ, ручная правка, история.

## Решения

- Backend: Go 1.23+, `net/http` + `go-chi/chi/v5`, `openai/openai-go` (official SDK), `jackc/pgx/v5` (pgxpool), миграции `pressly/goose` (embed), логи `log/slog`.
- DB: PostgreSQL 16.
- Frontend: Vite + React + TypeScript, без UI-кита, CSS modules, минималистичный дизайн. Роутинг `react-router`.
- Запуск: `docker-compose` — сервисы `db` (postgres) и `app` (Go-бинарь, отдаёт API и собранный фронт через `embed`/static dir). Порт `8080`.
- Auth нет (localhost only).
- Источник вакансии: публичный API `https://api.hh.ru/vacancies/{id}` (JSON), не HTML-скрейпинг. Нужен заголовок `User-Agent` (например `soprovod/1.0 (egorgorban239@gmail.com)`). Поддержать URL вида `hh.ru/vacancy/123`, `spb.hh.ru/vacancy/123?...`.
- Резюме и шаблон — файлы в репо (`resume/resume.md`, `resume/template.md`), путь задаётся env. Читаются при старте.
- Фильтр вакансий: интерфейс `Filter`, реализация `MockFilter` → всегда `{Pass: true, Reason: "mock"}`. Результат сохраняется в БД.
- Правка письма: пользователь редактирует текст, сохраняется в БД (`edited_text`, исходный `generated_text` не трогаем).

## Шаблон письма

База — `template.md`:

```
Добрый день!

Я %роль% с опытом %количество%+ лет. Основной стек: %стек%.
Сейчас ищу предложения связанные с %стек%, поэтому заинтересовала ваша вакансия.

Буду рад обсудить дальнейшие шаги!
```

Подтверждено: между абзацем про стек и финальной фразой вставляется блок из 1–3 коротких предложений с релевантным опытом из резюме (компания/проект + что сделал). Без выдуманных фактов, только из резюме. Стек в письме — пересечение стека вакансии и резюме, не весь стек.

## Генерация (OpenAI)

Один вызов Chat Completions / Responses API со structured output (JSON Schema):

```json
{
  "role": "Backend-разработчик",
  "years": 5,
  "stack": ["Go", "PostgreSQL", "Kafka"],
  "relevant_experience": [
    {"source": "Sberlabs", "text": "разрабатывал API-Gateway на Go ..."}
  ],
  "letter": "Добрый день! ..."
}
```

Системный промпт — правила: не упоминать нерелевантный опыт, стиль живого человека, без канцелярита и клише ("я уверен, что стану ценным членом команды"), без markdown, 500–900 символов, язык — язык вакансии. Модель — env `OPENAI_MODEL`, по умолчанию `gpt-5-mini` (nano путал факты). Сохраняем в БД весь JSON + имя модели.

## Схема БД

```sql
vacancies (
  id bigserial pk,
  hh_id text unique not null,
  url text not null,
  title text, company text, salary text,
  description text not null,          -- plain text, HTML вырезан
  key_skills text[],
  raw jsonb not null,                 -- ответ hh API
  created_at timestamptz default now()
);

applications (
  id bigserial pk,
  vacancy_id bigint not null references vacancies(id),
  filter_passed boolean not null,
  filter_reason text,
  status text not null,               -- 'generated' | 'filtered_out' | 'failed'
  generated_text text,
  edited_text text,
  llm_output jsonb,                   -- полный structured output
  model text,
  error text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);
```

Повторная отправка той же вакансии: переиспользуем `vacancies` по `hh_id`, создаём новую `applications`.

## API

- `POST /api/applications` `{url}` → `201 {application, vacancy}`. Синхронно (fetch + LLM ~5–20 c).
- `GET /api/applications?limit=&offset=` → список (id, title, company, status, created_at, первые 150 символов письма).
- `GET /api/applications/{id}` → полная запись + вакансия.
- `PATCH /api/applications/{id}` `{edited_text}` → сохранить правку.
- `GET /healthz`.

Ошибки: `{error: "..."}`, коды 400 (плохой URL), 404, 502 (hh/OpenAI недоступны).

## Структура

```
backend/
  cmd/server/main.go            wiring, graceful shutdown
  internal/config/              env
  internal/hh/                  ParseVacancyID(url), Client.GetVacancy(ctx,id), HTML→text
  internal/filter/              Filter interface, MockFilter
  internal/letter/              Generator interface, OpenAIGenerator, prompt.go
  internal/pipeline/            Service.Process(ctx,url) — fetch→filter→generate→save
  internal/storage/             Postgres repo, migrations/ (goose, embed)
  internal/httpapi/             chi router, handlers, static
frontend/
  src/api.ts                    типизированный клиент
  src/pages/NewApplication.tsx  поле URL → лоадер → письмо (textarea), копировать, сохранить
  src/pages/History.tsx         список
  src/pages/ApplicationView.tsx просмотр/правка + описание вакансии (свёрнуто)
resume/resume.md, resume/template.md
Dockerfile (multi-stage: node build → go build → distroless)
docker-compose.yml, .env.example, Makefile
```

`pipeline.Service` не знает про HTTP — будущий cron-воркер ("список вакансий → фильтр → письмо") вызовет тот же `Process`.

## План для агентов

Контракт между агентами — раздел «API» и «Схема БД» выше. Отступать нельзя без обновления этого файла.

### Волна 1 (параллельно)

**A1. Backend skeleton + storage**
- `go mod init`, config, main, chi router, `/healthz`, slog.
- goose миграции по схеме, pgxpool, репозиторий: `UpsertVacancy`, `CreateApplication`, `ListApplications`, `GetApplication`, `UpdateEditedText`.
- docker-compose (`db` + `app`), Dockerfile, `.env.example`, Makefile (`make up`, `make dev`, `make test`).
- Тесты репо на testcontainers или на compose-postgres.

**A2. hh клиент**
- `ParseVacancyID` (таблица тестов с разными URL).
- `GetVacancy` через api.hh.ru, User-Agent, timeout 10s, HTML описания → plain text (сохранить абзацы и списки).
- Тесты с `httptest.Server` + фикстура реального JSON.

**A3. Генератор письма**
- `Generator` интерфейс, `OpenAIGenerator` со structured output, промпт из резюме + шаблона + вакансии.
- `MockFilter`.
- Тест: мок OpenAI-сервера; ручной скрипт `cmd/try/main.go <url>` для проверки качества письма.

**A4. Frontend**
- Vite + React + TS, 3 страницы, api клиент по контракту, мок-режим пока бэк не готов.
- Минимализм: одна колонка ~680px, системный шрифт, светлая/тёмная тема, textarea с авторостом, кнопки «Скопировать» / «Сохранить».

### Волна 2

**A5. Pipeline + HTTP handlers**
- `pipeline.Service.Process`, статусы, сохранение ошибок.
- Handlers по контракту, отдача `frontend/dist`.
- Интеграционный тест: мок hh + мок OpenAI + реальный postgres.

### Волна 3

**A6. Сборка и проверка**
- Multi-stage Dockerfile с фронтом, `docker compose up` поднимает всё.
- E2E ручной прогон на 2–3 реальных вакансиях, правка промпта по результату.
- README: запуск, env.

## Будущее (не делаем сейчас)

- Реальный фильтр (LLM-оценка соответствия + жёсткие правила: зарплата, формат, город).
- Cron-воркер: поиск вакансий через `api.hh.ru/vacancies?text=...`, дедуп по `hh_id`, статус `new`.
- Перегенерация и версии писем.

## Статус промпта

Промпт в `backend/internal/letter/prompt.go` зафиксирован после ручной проверки на `gpt-5-mini`. Эталонное письмо для `cmd/try/testdata/go_backend_vacancy.txt` — `cmd/try/testdata/go_backend_vacancy.reference.txt`. При правках промпта сравнивать результат с эталоном.

## Режим «текст вакансии»

Второй способ ввода: пользователь вставляет текст вакансии вручную (hh сбоит или вакансия не с hh). Пайплайн тот же, шаг получения вакансии заменён.

- БД (новая миграция): `vacancies.source text not null default 'hh'` (`'hh' | 'manual'`); `hh_id` и `url` становятся nullable. Уникальность `hh_id` сохраняется для не-NULL значений. Ручные вакансии не дедуплицируются — каждая вставка создаёт новую строку.
- API: `POST /api/applications` принимает либо `{url}`, либо `{text, title?, company?}`. Оба поля сразу или ни одного — 400. Пустой `text` — 400. Лимит текста — 50 000 символов.
- Если `title` не передан, заголовок — первая непустая строка текста (обрезать до 120 символов).
- Pipeline: `ProcessText(ctx, ManualVacancy)` рядом с `Process(ctx, url)`, общий хвост (фильтр → генерация → сохранение) вынесен в одну функцию.
- Frontend: переключатель «Ссылка на hh» / «Текст вакансии» над полем ввода; в режиме текста — textarea и необязательные поля «Название», «Компания». Выбранный режим запоминается в localStorage. В карточке ручной вакансии нет ссылки на hh.
