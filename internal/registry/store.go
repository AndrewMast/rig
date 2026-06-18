package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Store persists a Registry to a single JSON file. Writes are atomic
// (temp + fsync + rename) and serialized by an exclusive flock on a sibling
// lock file, because the manifest is the rebuild source of truth and must never
// be observed half-written.
type Store struct {
	path string
}

// NewStore returns a Store backed by the given manifest path.
func NewStore(path string) *Store { return &Store{path: path} }

// Path returns the manifest file path.
func (s *Store) Path() string { return s.path }

// Load reads the manifest. A missing file yields a fresh empty registry (the
// first run). A schema-version mismatch is a hard error — there is no migration.
func (s *Store) Load() (*Registry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse registry %s: %w", s.path, err)
	}
	if r.Version != SchemaVersion {
		return nil, fmt.Errorf("registry schema v%d, want v%d (no migration; clean slate)", r.Version, SchemaVersion)
	}
	return &r, nil
}

// Save writes the manifest atomically. The directory is created if needed.
func (s *Store) Save(r *Registry) error {
	r.Version = SchemaVersion
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	unlock, err := lock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp registry: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp registry: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("commit registry: %w", err)
	}
	return nil
}

// lock takes an exclusive flock on the given lock-file path and returns a
// release function. The lock file is created if absent and left in place.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open registry lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock registry: %w", err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
