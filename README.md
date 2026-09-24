# Soprovod

Сервис для одного пользователя: вставляешь ссылку на вакансию hh.ru → сервис забирает
описание вакансии, прогоняет через LLM (OpenAI) и генерирует сопроводительное письмо на
основе резюме и шаблона, с релевантным опытом из резюме. Письмо можно править и хранить
историю всех откликов.

Без авторизации, рассчитан на локальный запуск (localhost).

## Быстрый старт (Docker)

1. Скопировать `.env.example` в `.env` и подставить свой `OPENAI_API_KEY`:

   ```sh
   cp .env.example .env
   # открыть .env и вписать OPENAI_API_KEY
   ```

2. Собрать и запустить:

   ```sh
   docker compose up --build
   # или
   make up
   ```

   Поднимаются два сервиса: `db` (Postgres 16) и `app` (Go-бинарь, отдаёт API и собранный
   фронтенд). `app` стартует только после того, как `db` станет healthy, и сам накатывает
   миграции при старте.

3. Открыть [http://localhost:8080](http://localhost:8080).

Остановить и снести данные:

```sh
docker compose down -v
```

## Makefile

- `make up` — `docker compose up --build`.
- `make down` — `docker compose down`.
- `make dev-db` — поднять только `db` (для локальной разработки бэкенда без Docker).
- `make run` — запустить бэкенд локально (`go run ./cmd/server`), без сборки образа.
- `make test` — тесты бэкенда (`TEST_DATABASE_URL=... make test` для тестов с реальной БД).

## Режим разработки

Бэкенд и фронтенд запускаются отдельно, с хот-релоадом.

1. Поднять только БД: `make dev-db`.
2. Бэкенд:

   ```sh
   cd backend
   export DATABASE_URL=postgres://soprovod:soprovod@localhost:5432/soprovod?sslmode=disable
   export OPENAI_API_KEY=...      # свой ключ
   go run ./cmd/server
   ```

   Слушает `:8080`, отдаёт `/healthz` и `/api/*`. `RESUME_PATH`/`TEMPLATE_PATH` по умолчанию
   `resume/resume.md` и `resume/template.md` относительно `backend/` — при запуске из
   каталога `backend/` их стоит переопределить (`../resume/...`) либо запускать из корня
   репозитория.

3. Фронтенд:

   ```sh
   cd frontend
   npm install
   npm run dev
   ```

   Vite слушает `:5173` и проксирует `/api/*` на `http://localhost:8080` (см.
   `frontend/vite.config.ts`), так что можно работать против реального бэкенда.

   Для работы без бэкенда есть мок-режим: скопировать `frontend/.env.local.example` в
   `frontend/.env.local` (`VITE_MOCK=1`) — фронтенд будет отдавать заглушечные данные вместо
   реальных запросов к API.

## `cmd/try` — проверка качества письма без БД и HTTP

Ручной скрипт, который берёт файл с вакансией (первая строка — заголовок, остальное —
описание), прогоняет его через `OpenAIGenerator` с резюме и шаблоном из репозитория и
печатает в stdout сырой JSON от модели плюс готовый текст письма. Удобно для быстрой
проверки промпта/качества без поднятия БД и API.

```sh
cd backend
export OPENAI_API_KEY=...
go run ./cmd/try ./cmd/try/testdata/go_backend_vacancy.txt
```

## Резюме и шаблон письма

Лежат в `resume/resume.md` и `resume/template.md` (пути настраиваются через
`RESUME_PATH` / `TEMPLATE_PATH`). Читаются один раз при старте сервера — если поменять
файлы, **нужно перезапустить** `app` (`docker compose restart app` или `make run` заново),
без пересборки образа, так как файлы монтируются/копируются, а не перечитываются на лету.

## Структура проекта

```
backend/
  cmd/server/       точка входа: конфиг, миграции, HTTP-сервер, graceful shutdown
  cmd/try/          ручной скрипт для проверки генерации письма
  internal/config/  чтение конфигурации из env
  internal/hh/      парсинг URL вакансии, клиент api.hh.ru, HTML → plain text
  internal/filter/  интерфейс Filter, MockFilter
  internal/letter/  интерфейс Generator, OpenAIGenerator, промпт
  internal/pipeline/ Service.Process — fetch → filter → generate → save
  internal/storage/ репозиторий на pgx, goose-миграции (embed)
  internal/httpapi/ chi-роутер, хендлеры, отдача статики фронтенда

frontend/
  src/api.ts                  типизированный клиент API (+ мок-режим)
  src/pages/NewApplication.tsx поле URL → письмо
  src/pages/History.tsx        список откликов
  src/pages/ApplicationView.tsx просмотр/правка письма + описание вакансии

resume/resume.md, resume/template.md   резюме и шаблон письма
Dockerfile           multi-stage: node build → go build → distroless
docker-compose.yml    db (postgres) + app
.env.example, Makefile
```

## API (контракт)

- `POST /api/applications` `{url}` → `201 {application, vacancy}`.
- `GET /api/applications?limit=&offset=` → список откликов.
- `GET /api/applications/{id}` → полная запись.
- `PATCH /api/applications/{id}` `{edited_text}` → сохранить правку письма.
- `GET /healthz`.

Подробности и схема БД — в `PLAN.md`.

## Дальнейшие планы

- Настоящий фильтр вакансий вместо `MockFilter` (сейчас всегда пропускает всё).
- Cron-воркер: список вакансий → фильтр → генерация письма без ручного ввода ссылки,
  поверх того же `pipeline.Service.Process`, который уже не завязан на HTTP.
