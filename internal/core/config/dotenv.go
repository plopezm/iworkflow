package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

const dotEnvFile = ".env"

// LoadDotEnv loads the first `.env` found in dir or one of its ancestors into the process environment,
// like the Rust server did. Variables that are already set win over the file. It returns the path that
// was loaded, or "" when no file exists.
func LoadDotEnv(dir string) (string, error) {
	for current := dir; ; current = filepath.Dir(current) {
		path := filepath.Join(current, dotEnvFile)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if err := godotenv.Load(path); err != nil {
				return "", fmt.Errorf("load %s: %w", path, err)
			}
			return path, nil
		}
		if filepath.Dir(current) == current {
			return "", nil
		}
	}
}
