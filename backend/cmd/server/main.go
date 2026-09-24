// Command server wires up config, storage and the HTTP API and runs the
// service until interrupted.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/egorgorban/soprovod/backend/internal/config"
	"github.com/egorgorban/soprovod/backend/internal/filter"
	"github.com/egorgorban/soprovod/backend/internal/hh"
	"github.com/egorgorban/soprovod/backend/internal/httpapi"
	"github.com/egorgorban/soprovod/backend/internal/letter"
	"github.com/egorgorban/soprovod/backend/internal/pipeline"
	"github.com/egorgorban/soprovod/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	if err := storage.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := storage.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := storage.NewRepo(pool)

	resume, err := os.ReadFile(cfg.ResumePath)
	if err != nil {
		return fmt.Errorf("read resume file %q: %w", cfg.ResumePath, err)
	}
	template, err := os.ReadFile(cfg.TemplatePath)
	if err != nil {
		return fmt.Errorf("read template file %q: %w", cfg.TemplatePath, err)
	}

	hhClient := hh.NewClient(nil, "", "")
	mockFilter := filter.MockFilter{}
	generator := letter.NewOpenAIGenerator(cfg.OpenAIAPIKey, cfg.OpenAIModel, string(resume), string(template))
	pl := pipeline.NewService(hhClient, mockFilter, generator, repo, logger)

	handler := httpapi.NewRouter(httpapi.Deps{
		Logger:    logger,
		Pipeline:  pl,
		Repo:      repo,
		StaticDir: cfg.StaticDir,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
