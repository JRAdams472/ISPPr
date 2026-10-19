package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Backend stores finished package artifacts by key. LocalStore serves dev;
// MPE-36 adds the S3/MinIO backend with presigned URLs — orchestration only
// talks to this interface.
type Backend interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// LocalStore writes artifacts under a filesystem root — the dev backend.
type LocalStore struct {
	Dir string
}

func (l *LocalStore) Put(_ context.Context, key string, r io.Reader) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (l *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// path resolves key under Dir, refusing traversal outside the root —
// reject separators at the start (POSIX- and Windows-absolute), drive
// letters, and ".." components before any platform normalization.
func (l *LocalStore) path(key string) (string, error) {
	if l.Dir == "" {
		return "", errors.New("storage dir not configured")
	}
	if key == "" || strings.HasPrefix(key, "/") || strings.HasPrefix(key, `\`) ||
		strings.Contains(key, ":") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	clean := filepath.Clean(strings.ReplaceAll(key, "/", string(filepath.Separator)))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(l.Dir, clean), nil
}
