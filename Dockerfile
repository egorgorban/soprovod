# syntax=docker/dockerfile:1

# --- frontend build (optional: skipped gracefully if frontend/ absent) ---
FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/ ./frontend/
RUN if [ -f frontend/package.json ]; then \
      cd frontend && npm ci && npm run build; \
    else \
      mkdir -p frontend/dist; \
    fi

# --- backend build ---
FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 go build -o /out/server ./cmd/server

# --- final image ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=backend /out/server ./server
COPY --from=frontend /src/frontend/dist ./frontend/dist
COPY resume/ ./resume/

ENV HTTP_ADDR=:8080 \
    STATIC_DIR=frontend/dist \
    RESUME_PATH=resume/resume.md \
    TEMPLATE_PATH=resume/template.md

EXPOSE 8080
ENTRYPOINT ["/app/server"]
