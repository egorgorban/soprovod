// Command fetch is a manual script for exercising hhparser.Client against a
// real hh.ru vacancy page. It either prints the parsed hh.Vacancy as
// indented JSON, or (with -save) dumps the raw HTML response body to a file
// for later use as a test fixture.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/egorgorban/soprovod/backend/internal/hh"
	"github.com/egorgorban/soprovod/backend/internal/hhparser"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	savePath := flag.String("save", "", "instead of parsing, GET the vacancy page and write the raw HTML body to this path")
	flag.Parse()

	if flag.NArg() < 1 {
		return fmt.Errorf("usage: fetch [-save path] <hh.ru-vacancy-url-or-id>")
	}
	arg := flag.Arg(0)

	id, err := hh.ParseVacancyID(arg)
	if err != nil {
		return fmt.Errorf("parse vacancy id from %q: %w", arg, err)
	}

	if *savePath != "" {
		return saveRaw(id, *savePath)
	}

	c := hhparser.NewClient(&http.Client{Timeout: 15 * time.Second}, "")

	v, err := c.GetVacancy(context.Background(), id)
	if err != nil {
		return fmt.Errorf("get vacancy %s: %w", id, err)
	}

	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal vacancy: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// saveRaw fetches the raw hh.ru vacancy page for id (no parsing) and writes
// the response body to path, for manually capturing test fixtures.
func saveRaw(id, path string) error {
	url := fmt.Sprintf("https://hh.ru/vacancy/%s", id)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	fmt.Printf("saved %d bytes (status %d) to %s\n", n, resp.StatusCode, path)
	return nil
}
