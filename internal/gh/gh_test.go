package gh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTokenEnvWins(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	os.WriteFile(file, []byte("file-token\n"), 0o600)

	t.Setenv("RIG_GH_TOKEN", "env-token")
	if got := LoadToken(file); got != "env-token" {
		t.Errorf("token = %q, want env-token", got)
	}

	t.Setenv("RIG_GH_TOKEN", "")
	if got := LoadToken(file); got != "file-token" {
		t.Errorf("token = %q, want file-token", got)
	}

	if got := LoadToken(filepath.Join(dir, "absent")); got != "" {
		t.Errorf("missing token file should yield empty, got %q", got)
	}
}

func TestAvailable(t *testing.T) {
	if New("").Available() {
		t.Error("empty token should be unavailable")
	}
	if !New("tok").Available() {
		t.Error("token should be available")
	}
}
