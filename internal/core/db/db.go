// Package db opens the SQLite database and applies the embedded migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// connectionPragmas are applied to every new SQLite connection.
var connectionPragmas = []string{
	"foreign_keys(1)",
	"journal_mode(WAL)",
	"busy_timeout(5000)",
	"synchronous(NORMAL)",
	"temp_store(MEMORY)",
}

// Open opens (creating the parent folder if needed) the SQLite database at path.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database folder: %w", err)
		}
	}
	handle, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := handle.PingContext(ctx); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return handle, nil
}

func dsn(path string) string {
	query := url.Values{}
	for _, pragma := range connectionPragmas {
		query.Add("_pragma", pragma)
	}
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + query.Encode()
}
